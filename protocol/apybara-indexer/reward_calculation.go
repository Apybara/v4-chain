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

	//AnnualizeReward500kb
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	annualizedRewards500kb, err := r.AnnualizedRewards(ctx.BlockHeight(), denom)
	if err != nil {
		fmt.Println("error calculating annualizedRewards500kb", err)
		return 0, err
	}
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.18f", annualizedRewards500kb)
	database.Save(&rewardDataDelta)
	return rewardDelta, nil
}

func (r RewardCalculatorService) AnnualizedRewards(blockHeight int64, denom string) (float64, error) {
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	db := r.Database
	var rewardDataDelta RewardDataDelta
	var annualizedRewards500kb float64
	var timestamp500kBlocks int64
	db.Model(&rewardDataDelta).Where("block_height = ? AND denom = ?", blockHeight, denom).First(&rewardDataDelta)
	//SELECT SUM(cast(delta AS DOUBLE PRECISION)) FROM reward_data_delta WHERE block_height < 2860685 - 1 AND block_height >= 2860685 - 500000 and denom = 'adydx';
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	db.Raw("SELECT timestamp FROM reward_data_delta WHERE block_height < ? - 1 AND block_height >= ? - 500000 and denom = ? ORDER BY timestamp DESC LIMIT 1", blockHeight, blockHeight, denom).Scan(&timestamp500kBlocks)
	db.Raw("SELECT SUM(cast(delta AS DOUBLE PRECISION)) FROM reward_data_delta WHERE block_height < ? - 1 AND block_height >= ? - 500000 and denom = ?", blockHeight, blockHeight, denom).Scan(&annualizedRewards500kb)
	annualizedRewards500kb = annualizedRewards500kb / float64(timestamp500kBlocks) * 365 * 24 * 60 * 60
	fmt.Sprintf("%.6f", annualizedRewards500kb)
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.18f", annualizedRewards500kb)

	return annualizedRewards500kb, nil
}
