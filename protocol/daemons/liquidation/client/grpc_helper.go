package client

import (
	"context"
	"fmt"
	"time"

	gometrics "github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/dydxprotocol/v4-chain/protocol/daemons/liquidation/api"
	"github.com/dydxprotocol/v4-chain/protocol/lib/metrics"
	blocktimetypes "github.com/dydxprotocol/v4-chain/protocol/x/blocktime/types"
	clobtypes "github.com/dydxprotocol/v4-chain/protocol/x/clob/types"
	pricestypes "github.com/dydxprotocol/v4-chain/protocol/x/prices/types"
	satypes "github.com/dydxprotocol/v4-chain/protocol/x/subaccounts/types"
	"google.golang.org/grpc/metadata"
)

// GetPreviousBlockInfo queries a gRPC server using `QueryPreviousBlockInfoRequest`
// and returns the previous block height.
func (c *Client) GetPreviousBlockInfo(
	ctx context.Context,
) (
	blockHeight uint32,
	err error,
) {
	defer metrics.ModuleMeasureSince(
		metrics.LiquidationDaemon,
		metrics.DaemonGetPreviousBlockInfoLatency,
		time.Now(),
	)

	query := &blocktimetypes.QueryPreviousBlockInfoRequest{}
	response, err := c.BlocktimeQueryClient.PreviousBlockInfo(ctx, query)
	if err != nil {
		return 0, err
	}

	return response.Info.Height, nil
}

// GetAllMarketPrices queries gRPC server and returns a list of market prices.
func (c *Client) GetAllMarketPrices(
	ctx context.Context,
	blockHeight uint32,
	pageLimit uint64,
) (
	marketPrices []pricestypes.MarketPrice,
	err error,
) {
	defer metrics.ModuleMeasureSince(
		metrics.LiquidationDaemon,
		metrics.DaemonGetAllMarketPricesLatency,
		time.Now(),
	)

	marketPrices = make([]pricestypes.MarketPrice, 0)

	// Set the block height header to the block height of the previous block.
	ctx = metadata.NewOutgoingContext(
		ctx,
		metadata.Pairs(
			grpc.GRPCBlockHeightHeader,
			fmt.Sprintf("%d", blockHeight),
		),
	)

	var nextKey []byte
	for {
		marketPricesFromKey, next, err := getMarketPricesFromKey(
			ctx,
			c.PricesQueryClient,
			nextKey,
			pageLimit,
		)

		if err != nil {
			return nil, err
		}

		marketPrices = append(marketPrices, marketPricesFromKey...)
		nextKey = next

		if len(nextKey) == 0 {
			break
		}
	}
	return marketPrices, nil
}

// GetAllSubaccounts queries a gRPC server and returns a list of subaccounts and
// their balances and open positions.
func (c *Client) GetAllSubaccounts(
	ctx context.Context,
	pageLimit uint64,
) (
	subaccounts []satypes.Subaccount,
	err error,
) {
	defer telemetry.ModuleMeasureSince(metrics.LiquidationDaemon, time.Now(), metrics.GetAllSubaccounts, metrics.Latency)
	subaccounts = make([]satypes.Subaccount, 0)

	var nextKey []byte
	for {
		subaccountsFromKey, next, err := getSubaccountsFromKey(
			ctx,
			c.SubaccountQueryClient,
			nextKey,
			pageLimit,
		)

		if err != nil {
			return nil, err
		}

		subaccounts = append(subaccounts, subaccountsFromKey...)
		nextKey = next

		if len(nextKey) == 0 {
			break
		}
	}

	telemetry.ModuleSetGauge(
		metrics.LiquidationDaemon,
		float32(len(subaccounts)),
		metrics.GetAllSubaccounts,
		metrics.Count,
	)

	return subaccounts, nil
}

// CheckCollateralizationForSubaccounts queries a gRPC server using `AreSubaccountsLiquidatable`
// and returns a list of collateralization statuses for the given list of subaccount ids.
func (c *Client) CheckCollateralizationForSubaccounts(
	ctx context.Context,
	subaccountIds []satypes.SubaccountId,
) (
	results []clobtypes.AreSubaccountsLiquidatableResponse_Result,
	err error,
) {
	defer telemetry.ModuleMeasureSince(
		metrics.LiquidationDaemon,
		time.Now(),
		metrics.CheckCollateralizationForSubaccounts,
		metrics.Latency,
	)

	query := &clobtypes.AreSubaccountsLiquidatableRequest{
		SubaccountIds: subaccountIds,
	}
	response, err := c.ClobQueryClient.AreSubaccountsLiquidatable(ctx, query)
	if err != nil {
		return nil, err
	}

	return response.Results, nil
}

// SendLiquidatableSubaccountIds sends a list of unique and potentially liquidatable
// subaccount ids to a gRPC server via `LiquidateSubaccounts`.
func (c *Client) SendLiquidatableSubaccountIds(
	ctx context.Context,
	subaccountIds []satypes.SubaccountId,
) error {
	defer telemetry.ModuleMeasureSince(
		metrics.LiquidationDaemon,
		time.Now(),
		metrics.SendLiquidatableSubaccountIds,
		metrics.Latency,
	)

	telemetry.ModuleSetGauge(
		metrics.LiquidationDaemon,
		float32(len(subaccountIds)),
		metrics.LiquidatableSubaccountIds,
		metrics.Count,
	)

	request := &api.LiquidateSubaccountsRequest{
		SubaccountIds: subaccountIds,
	}

	if _, err := c.LiquidationServiceClient.LiquidateSubaccounts(ctx, request); err != nil {
		return err
	}
	return nil
}

func getSubaccountsFromKey(
	ctx context.Context,
	client satypes.QueryClient,
	pageRequestKey []byte,
	limit uint64,
) (
	subaccounts []satypes.Subaccount,
	nextKey []byte,
	err error,
) {
	defer metrics.ModuleMeasureSinceWithLabels(
		metrics.LiquidationDaemon,
		[]string{metrics.GetSubaccountsFromKey, metrics.Latency},
		time.Now(),
		[]gometrics.Label{
			metrics.GetLabelForIntValue(metrics.PageLimit, int(limit)),
		},
	)

	query := &satypes.QueryAllSubaccountRequest{
		Pagination: &query.PageRequest{
			Key:   pageRequestKey,
			Limit: limit,
		},
	}

	response, err := client.SubaccountAll(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	if response.Pagination != nil {
		nextKey = response.Pagination.NextKey
	}
	return response.Subaccount, nextKey, nil
}

func getMarketPricesFromKey(
	ctx context.Context,
	client pricestypes.QueryClient,
	pageRequestKey []byte,
	limit uint64,
) (
	marketPrices []pricestypes.MarketPrice,
	nextKey []byte,
	err error,
) {
	defer metrics.ModuleMeasureSince(
		metrics.LiquidationDaemon,
		metrics.DaemonGetMarketPricesPaginatedLatency,
		time.Now(),
	)

	query := &pricestypes.QueryAllMarketPricesRequest{
		Pagination: &query.PageRequest{
			Key:   pageRequestKey,
			Limit: limit,
		},
	}

	response, err := client.AllMarketPrices(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	if response.Pagination != nil {
		nextKey = response.Pagination.NextKey
	}
	return response.MarketPrices, nextKey, nil
}
