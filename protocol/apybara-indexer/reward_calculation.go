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
