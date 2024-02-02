package apybara_indexer

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"
	"strconv"
)

type RewardCalculatorService struct {
	Database *gorm.DB
}

type BlockerAmount struct {
	BeforeBeginBlocker types.Dec
	AfterBeginBlocker  types.Dec
	Denom              string
}

func (r RewardCalculatorService) RewardDeltaForBlockers(ctx types.Context, blocker BlockerAmount, database *gorm.DB) error {

	afterBeginBlocker, err := strconv.ParseFloat(blocker.AfterBeginBlocker.String(), 64)
	if err != nil {
		fmt.Println("error parsing float for afterBeginBlocker", err)
		return err
	}

	beforeBeginBlocker, err := strconv.ParseFloat(blocker.BeforeBeginBlocker.String(), 64)
	if err != nil {
		fmt.Println("error parsing float for beforeBeginBlocker", err)
		return err
	}

	rewardDelta := afterBeginBlocker - beforeBeginBlocker

	//
	var rewardDataDelta RewardDataDelta
	rewardDataDelta.AfterBeginBlockerAmount = blocker.AfterBeginBlocker.String()
	rewardDataDelta.BeforeBeginBlockerAmount = blocker.BeforeBeginBlocker.String()
	rewardDataDelta.Denom = blocker.Denom
	rewardDataDelta.Timestamp = ctx.BlockTime().Unix()
	rewardDataDelta.BlockHeight = ctx.BlockHeight()
	rewardDataDelta.Delta = fmt.Sprintf("%.18f", rewardDelta)
	database.Create(&rewardDataDelta)
	//}
	return nil
}

func (r RewardCalculatorService) RewardDelta(ctx types.Context, denom string) (float64, error) {

	//AfterBeginBlocker
	var totalRewardAfterBeginBlocker TotalReward
	database := r.Database
	database.Where("block_height = ? AND denom = ? and event_type = ?", ctx.BlockHeight(), denom, "AfterBeginBlocker").First(&totalRewardAfterBeginBlocker)

	// BeforeBeginBlocker
	var totalRewardBeforeBeginBlocker TotalReward
	database.Where("block_height = ? AND denom = ? and event_type = ?", ctx.BlockHeight(), denom, "BeforeBeginBlocker").First(&totalRewardBeforeBeginBlocker)

	rewardAfterBeginBlocker, err := strconv.ParseFloat(totalRewardAfterBeginBlocker.Amount, 64)
	if err != nil {
		fmt.Println("error parsing float for rewardAfterBeginBlocker", err)
		return 0, err
	}
	rewardBeforeBeginBlocker, err := strconv.ParseFloat(totalRewardBeforeBeginBlocker.Amount, 64)
	if err != nil {
		fmt.Println("error parsing float for rewardBeforeBeginBlocker", err)
		return 0, err
	}
	//		blockValidatorData.Network75kb = fmt.Sprintf("%.6f", averageNetwork1b)
	var rewardDelta float64
	rewardDelta = rewardAfterBeginBlocker - rewardBeforeBeginBlocker

	//
	var rewardDataDelta RewardDataDelta
	rewardDataDelta.AfterBeginBlockerAmount = totalRewardAfterBeginBlocker.Amount
	rewardDataDelta.BeforeBeginBlockerAmount = totalRewardBeforeBeginBlocker.Amount
	rewardDataDelta.Denom = denom
	rewardDataDelta.Timestamp = ctx.BlockTime().Unix()
	rewardDataDelta.BlockHeight = ctx.BlockHeight()
	rewardDataDelta.Delta = fmt.Sprintf("%.18f", rewardDelta)
	database.Create(&rewardDataDelta)
	return rewardDelta, nil
}
