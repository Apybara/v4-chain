package keeper_test

import (
	"fmt"
	"github.com/cometbft/cometbft/libs/log"
	cometbfttypes "github.com/cometbft/cometbft/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/dydxprotocol/v4-chain/protocol/mocks"
	testapp "github.com/dydxprotocol/v4-chain/protocol/testutil/app"
	"github.com/dydxprotocol/v4-chain/protocol/testutil/constants"
	"github.com/dydxprotocol/v4-chain/protocol/testutil/delaymsg"
	keepertest "github.com/dydxprotocol/v4-chain/protocol/testutil/keeper"
	sdktest "github.com/dydxprotocol/v4-chain/protocol/testutil/sdk"
	bridgetypes "github.com/dydxprotocol/v4-chain/protocol/x/bridge/types"
	"github.com/dydxprotocol/v4-chain/protocol/x/delaymsg/keeper"
	"github.com/dydxprotocol/v4-chain/protocol/x/delaymsg/types"
	feetierstypes "github.com/dydxprotocol/v4-chain/protocol/x/feetiers/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	BridgeAuthority      = authtypes.NewModuleAddress(bridgetypes.ModuleName).String()
	BridgeAccountAddress = sdk.MustAccAddressFromBech32(BridgeAuthority)

	DelayMsgAuthority = authtypes.NewModuleAddress(types.ModuleName).String()

	BridgeGenesisAccountBalance = sdk.NewCoin("dv4tnt", sdk.NewInt(1000000000))
	AliceInitialAccountBalance  = sdk.NewCoin("dv4tnt", sdk.NewInt(99500000000))

	delta                        = constants.BridgeEvent_Id0_Height0.Coin.Amount.Int64()
	BridgeExpectedAccountBalance = sdk.NewCoin("dv4tnt", sdk.NewInt(1000000000-delta))
	AliceExpectedAccountBalance  = sdk.NewCoin("dv4tnt", sdk.NewInt(99500000000+delta))
)

func TestDispatchMessagesForBlock(t *testing.T) {
	ctx, k, _, bridgeKeeper, _ := keepertest.DelayMsgKeeperWithMockBridgeKeeper(t)

	// Add messages to the keeper.
	for i, msg := range constants.AllMsgs {
		id, err := k.DelayMessageByBlocks(ctx, msg, 0)
		require.NoError(t, err)
		require.Equal(t, uint32(i), id)
	}

	// Sanity check: messages appear for block 0.
	blockMessageIds, found := k.GetBlockMessageIds(ctx, 0)
	require.True(t, found)
	require.Equal(t, []uint32{0, 1, 2}, blockMessageIds.Ids)

	// Mock the bridge keeper methods called by the bridge msg server.
	bridgeKeeper.On("CompleteBridge", ctx, mock.Anything).Return(nil).Times(len(constants.AllMsgs))
	bridgeKeeper.On("HasAuthority", DelayMsgAuthority).Return(true).Times(len(constants.AllMsgs))

	// Dispatch messages for block 0.

	keeper.DispatchMessagesForBlock(k, ctx)

	_, found = k.GetBlockMessageIds(ctx, 0)
	require.False(t, found)

	require.True(t, bridgeKeeper.AssertExpectations(t))
}

func setupMockKeeperNoMessages(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper) {
	k.On("GetBlockMessageIds", ctx, int64(0)).Return(types.BlockMessageIds{}, false).Once()
}

func HandlerSuccess(_ sdk.Context, _ sdk.Msg) (*sdk.Result, error) {
	return &sdk.Result{}, nil
}

func HandlerFailure(_ sdk.Context, _ sdk.Msg) (*sdk.Result, error) {
	return &sdk.Result{}, fmt.Errorf("failed to handle message")
}

// mockSuccessRouter returns a handler that succeeds on all calls.
func mockSuccessRouter(_ sdk.Context) *mocks.MsgRouter {
	router := &mocks.MsgRouter{}
	router.On("Handler", mock.Anything).Return(HandlerSuccess).Times(3)
	return router
}

// mockFailingRouter returns a handler that fails on the first call, then returns a handler
// that succeeds on the next two.
func mockFailingRouter(ctx sdk.Context) *mocks.MsgRouter {
	router := mocks.MsgRouter{}
	router.On("Handler", mock.Anything).Return(HandlerFailure).Once()
	router.On("Handler", mock.Anything).Return(HandlerSuccess).Times(2)
	return &router
}

func setupMockKeeperMessageNotFound(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper) {
	k.On("GetBlockMessageIds", ctx, int64(0)).Return(types.BlockMessageIds{
		Ids: []uint32{0, 1, 2},
	}, true).Once()

	// Second message is not found.
	k.On("GetMessage", ctx, uint32(0)).Return(types.DelayedMessage{
		Id:          0,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg1),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(1)).Return(types.DelayedMessage{}, false).Once()
	k.On("GetMessage", ctx, uint32(2)).Return(types.DelayedMessage{
		Id:          2,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg3),
		BlockHeight: 0,
	}, true).Once()

	// 2 messages are routed.
	msgRouter := mockSuccessRouter(ctx)
	k.On("Router").Return(msgRouter).Times(2)

	// For error logging.
	k.On("Logger", ctx).Return(log.NewNopLogger()).Times(1)

	// All deletes are called.
	k.On("DeleteMessage", ctx, uint32(0)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(1)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(2)).Return(nil).Once()
}

func setupMockKeeperExecutionFailure(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper) {
	k.On("GetBlockMessageIds", ctx, int64(0)).Return(types.BlockMessageIds{
		Ids: []uint32{0, 1, 2},
	}, true).Once()

	// All messages found.
	k.On("GetMessage", ctx, uint32(0)).Return(types.DelayedMessage{
		Id:          0,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg1),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(1)).Return(types.DelayedMessage{
		Id:          1,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg2),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(2)).Return(types.DelayedMessage{
		Id:          2,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg3),
		BlockHeight: 0,
	}, true).Once()

	// 1st message fails to execute. Following 2 succeed.
	successRouter := mockSuccessRouter(ctx)
	failureRouter := mockFailingRouter(ctx)
	k.On("Router").Return(failureRouter).Times(1)
	k.On("Router").Return(successRouter).Times(2)

	// For error logging.
	k.On("Logger", ctx).Return(log.NewNopLogger()).Times(1)

	// All deletes are called.
	k.On("DeleteMessage", ctx, uint32(0)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(1)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(2)).Return(nil).Once()
}

func setupMockKeeperDecodeFailure(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper) {
	k.On("GetBlockMessageIds", ctx, int64(0)).Return(types.BlockMessageIds{
		Ids: []uint32{0, 1, 2},
	}, true).Once()

	nonMsgAnyProto, err := codectypes.NewAnyWithValue(&types.BlockMessageIds{})
	require.NoError(t, err)

	// All messages found.
	k.On("GetMessage", ctx, uint32(0)).Return(types.DelayedMessage{
		Id:          0,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg1),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(1)).Return(types.DelayedMessage{
		Id:          1,
		Msg:         nonMsgAnyProto,
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(2)).Return(types.DelayedMessage{
		Id:          2,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg3),
		BlockHeight: 0,
	}, true).Once()

	// 2 messages are routed.
	k.On("Router").Return(mockSuccessRouter(ctx)).Times(2)

	// For error logging.
	k.On("Logger", ctx).Return(log.NewNopLogger()).Times(1)

	// All deletes are called. 2nd delete fails.
	k.On("DeleteMessage", ctx, uint32(0)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(1)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(2)).Return(nil).Once()
}

func setupMockKeeperDeletionFailure(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper) {
	k.On("GetBlockMessageIds", ctx, int64(0)).Return(types.BlockMessageIds{
		Ids: []uint32{0, 1, 2},
	}, true).Once()

	// All messages found.
	k.On("GetMessage", ctx, uint32(0)).Return(types.DelayedMessage{
		Id:          0,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg1),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(1)).Return(types.DelayedMessage{
		Id:          1,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg2),
		BlockHeight: 0,
	}, true).Once()
	k.On("GetMessage", ctx, uint32(2)).Return(types.DelayedMessage{
		Id:          2,
		Msg:         delaymsg.EncodeMessageToAny(t, constants.TestMsg3),
		BlockHeight: 0,
	}, true).Once()

	// All messages are routed.
	k.On("Router").Return(mockSuccessRouter(ctx)).Times(3)

	// For error logging.
	k.On("Logger", ctx).Return(log.NewNopLogger()).Times(1)

	// All deletes are called. 2nd delete fails.
	k.On("DeleteMessage", ctx, uint32(0)).Return(nil).Once()
	k.On("DeleteMessage", ctx, uint32(1)).Return(fmt.Errorf("Deletion failure")).Once()
	k.On("DeleteMessage", ctx, uint32(2)).Return(nil).Once()
}

func TestDispatchMessageForBlock_Mixed(t *testing.T) {
	tests := map[string]struct {
		setupMocks func(t *testing.T, ctx sdk.Context, k *mocks.DelayMsgKeeper)
	}{
		"No messages - dispatch terminates with no action": {
			setupMocks: setupMockKeeperNoMessages,
		},
		"Unexpected message not found does not affect remaining messages": {
			setupMocks: setupMockKeeperMessageNotFound,
		},
		"Execution error does not affect remaining messages": {
			setupMocks: setupMockKeeperExecutionFailure,
		},
		"Decode failure does not affect remaining messages": {
			setupMocks: setupMockKeeperDecodeFailure,
		},
		"Deletion failure does not affect deletion of remaining messages": {
			setupMocks: setupMockKeeperDeletionFailure,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			k := &mocks.DelayMsgKeeper{}
			ctx := sdktest.NewContextWithBlockHeightAndTime(0, time.Now())
			tc.setupMocks(t, ctx, k)

			keeper.DispatchMessagesForBlock(k, ctx)

			k.AssertExpectations(t)
		})
	}
}

// generateBridgeEventMsgAny wraps bridge event in a MsgCompleteBridge and encodes it into an Any.
func generateBridgeEventMsgAny(t *testing.T, event bridgetypes.BridgeEvent) *codectypes.Any {
	msgCompleteBridge := bridgetypes.MsgCompleteBridge{
		Authority: authtypes.NewModuleAddress(types.ModuleName).String(),
		Event:     event,
	}
	any, err := codectypes.NewAnyWithValue(&msgCompleteBridge)
	require.NoError(t, err)
	return any
}

// expectAccountBalance checks that the specified account has the expected balance.
func expectAccountBalance(
	t *testing.T,
	ctx sdk.Context,
	tApp *testapp.TestApp,
	address sdk.AccAddress,
	expectedBalance sdk.Coin,
) {
	balance := tApp.App.BankKeeper.GetBalance(ctx, address, expectedBalance.Denom)
	require.Equal(t, expectedBalance.Amount, balance.Amount)
	require.Equal(t, expectedBalance.Denom, balance.Denom)
}

func TestSendDelayedCompleteBridgeMessage(t *testing.T) {
	// Create an encoded bridge event set to occur at block 2.
	// Expect that Alice's account will increase by 888 coins at block 2.
	// Bridge module account will also decrease by 888 coins at block 2.
	delayedMessage := types.DelayedMessage{
		Id:          0,
		Msg:         generateBridgeEventMsgAny(t, constants.BridgeEvent_Id0_Height0),
		BlockHeight: 2,
	}

	tApp := testapp.NewTestAppBuilder().WithGenesisDocFn(func() (genesis cometbfttypes.GenesisDoc) {
		genesis = testapp.DefaultGenesis()
		// Add the delayed message to the genesis state.
		testapp.UpdateGenesisDocWithAppStateForModule(
			&genesis,
			func(genesisState *types.GenesisState) {
				genesisState.DelayedMessages = []*types.DelayedMessage{&delayedMessage}
				genesisState.NumMessages = 1
			},
		)
		return genesis
	}).WithTesting(t).Build()
	ctx := tApp.InitChain()

	// Sanity check: the delayed message is in the keeper scheduled for block 2.
	blockMessageIds, found := tApp.App.DelayMsgKeeper.GetBlockMessageIds(ctx, 2)
	require.True(t, found)
	require.Equal(t, []uint32{0}, blockMessageIds.Ids)

	aliceAccountAddress := sdk.MustAccAddressFromBech32(constants.BridgeEvent_Id0_Height0.Address)

	// Sanity check: at block 1, balances are as expected before the message is sent.
	expectAccountBalance(t, ctx, &tApp, BridgeAccountAddress, BridgeGenesisAccountBalance)
	expectAccountBalance(t, ctx, &tApp, aliceAccountAddress, AliceInitialAccountBalance)

	// Advance to block 2 and invoke delayed message to complete bridge.
	ctx = tApp.AdvanceToBlock(2, testapp.AdvanceToBlockOptions{})

	// Assert: balances have been updated to reflect the executed CompleteBridge message.
	expectAccountBalance(t, ctx, &tApp, BridgeAccountAddress, BridgeExpectedAccountBalance)
	expectAccountBalance(t, ctx, &tApp, aliceAccountAddress, AliceExpectedAccountBalance)

	// Assert: the message has been deleted from the keeper.
	_, found = tApp.App.DelayMsgKeeper.GetMessage(ctx, 0)
	require.False(t, found)

	// The block message ids have also been deleted.
	_, found = tApp.App.DelayMsgKeeper.GetBlockMessageIds(ctx, 2)
	require.False(t, found)
}

// TestSendDelayedPerpetualFeeParamsUpdate tests that the delayed message testApp genesis state, which contains a
// message to update the x/feetiers perpetual fee params after ~120 days of blocks, is executed correctly. In this
// test, we modify the genesis state to apply the parameter update on block 2 to validate that the update is applied
// correctly.
func TestSendDelayedPerpetualFeeParamsUpdate(t *testing.T) {
	tApp := testapp.NewTestAppBuilder().WithGenesisDocFn(func() (genesis cometbfttypes.GenesisDoc) {
		genesis = testapp.DefaultGenesis()
		// Update the genesis state to execute the perpetual fee params update at block 2.
		testapp.UpdateGenesisDocWithAppStateForModule(
			&genesis,
			func(genesisState *types.GenesisState) {
				// Update the default state to apply the first delayed message on block 2.
				// This is the PerpetualFeeParamsUpdate message.
				genesisState.DelayedMessages[0].BlockHeight = 2
			},
		)
		return genesis
	}).WithTesting(t).Build()
	ctx := tApp.InitChain()

	resp, err := tApp.App.FeeTiersKeeper.PerpetualFeeParams(ctx, &feetierstypes.QueryPerpetualFeeParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, feetierstypes.PromotionalParams(), resp.Params)

	// Advance to block 2 and invoke delayed message to complete bridge.
	ctx = tApp.AdvanceToBlock(3, testapp.AdvanceToBlockOptions{})

	resp, err = tApp.App.FeeTiersKeeper.PerpetualFeeParams(ctx, &feetierstypes.QueryPerpetualFeeParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, feetierstypes.StandardParams(), resp.Params)
}

func TestSendDelayedCompleteBridgeMessage_Failure(t *testing.T) {
	// Create an encoded bridge event set to occur at block 2.
	// The bridge event is invalid and will not execute.
	// Expect no account balance changes, and the message to be deleted from the keeper.
	invalidBridgeEvent := bridgetypes.BridgeEvent{
		Id:             constants.BridgeEvent_Id0_Height0.Id,
		Address:        "INVALID",
		Coin:           constants.BridgeEvent_Id0_Height0.Coin,
		EthBlockHeight: constants.BridgeEvent_Id0_Height0.EthBlockHeight,
	}
	_, err := sdk.AccAddressFromBech32("INVALID")
	require.Error(t, err)

	delayedMessage := types.DelayedMessage{
		Id:          0,
		Msg:         generateBridgeEventMsgAny(t, invalidBridgeEvent),
		BlockHeight: 2,
	}

	tApp := testapp.NewTestAppBuilder().WithGenesisDocFn(func() (genesis cometbfttypes.GenesisDoc) {
		genesis = testapp.DefaultGenesis()
		// Add the delayed message to the genesis state.
		testapp.UpdateGenesisDocWithAppStateForModule(
			&genesis,
			func(genesisState *types.GenesisState) {
				genesisState.DelayedMessages = []*types.DelayedMessage{&delayedMessage}
				genesisState.NumMessages = 1
			},
		)
		return genesis
	}).WithTesting(t).Build()
	ctx := tApp.InitChain()

	// Sanity check: at block 1, balances are as expected before the message is sent.
	expectAccountBalance(t, ctx, &tApp, BridgeAccountAddress, BridgeGenesisAccountBalance)

	// Sanity check: a message with this id exists within the keeper.
	_, found := tApp.App.DelayMsgKeeper.GetMessage(ctx, 0)
	require.True(t, found)

	// Sanity check: this message id is scheduled to be executed at block 2.
	messageIds, found := tApp.App.DelayMsgKeeper.GetBlockMessageIds(ctx, 2)
	require.True(t, found)
	require.Equal(t, []uint32{0}, messageIds.Ids)

	// Advance to block 2 and invoke delayed message to complete bridge.
	ctx = tApp.AdvanceToBlock(2, testapp.AdvanceToBlockOptions{})

	// Assert: balances have been updated to reflect the executed CompleteBridge message.
	expectAccountBalance(t, ctx, &tApp, BridgeAccountAddress, BridgeGenesisAccountBalance)

	// Assert: the message has been deleted from the keeper.
	_, found = tApp.App.DelayMsgKeeper.GetMessage(ctx, 0)
	require.False(t, found)

	// The block message ids have also been deleted.
	_, found = tApp.App.DelayMsgKeeper.GetBlockMessageIds(ctx, 2)
	require.False(t, found)
}