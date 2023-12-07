package apybara_indexer

import (
	"fmt"
	"gorm.io/gorm"
	"strconv"
)

type RewardCalculatorService struct {
	Database *gorm.DB
}

func (r RewardCalculatorService) RewardDelta(blockHeight int64, denom string) (float64, error) {

	//AfterBeginBlocker
	var totalRewardAfterBeginBlocker TotalReward
	database := r.Database
	database.Where("block_height = ? AND denom = ? and type = ?", blockHeight, denom, "AfterBeginBlocker").First(&totalRewardAfterBeginBlocker)

	// BeforeBeginBlocker
	var totalRewardBeforeBeginBlocker TotalReward
	database.Where("block_height = ? AND denom = ? and type = ?", blockHeight, denom, "BeforeBeginBlocker").First(&totalRewardBeforeBeginBlocker)

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
	rewardDataDelta.BlockHeight = blockHeight
	rewardDataDelta.Delta = fmt.Sprintf("%.6f", rewardDelta)
	database.Create(&rewardDataDelta)

	//AnnualizeReward500kb
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	annualizedRewards500kb, err := r.AnnualizedRewards(blockHeight, denom)
	if err != nil {
		fmt.Println("error calculating annualizedRewards500kb", err)
		return 0, err
	}
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.6f", annualizedRewards500kb)
	database.Save(&rewardDataDelta)
	return rewardDelta, nil
}

func (r RewardCalculatorService) AnnualizedRewards(blockHeight int64, denom string) (float64, error) {
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	db := r.Database
	var rewardDataDelta RewardDataDelta
	var annualizedRewards500kb float64
	db.Model(&rewardDataDelta).Where("block_height = ? AND denom = ?", blockHeight, denom).First(&rewardDataDelta)

	db.Raw("SELECT SUM(delta) FROM reward_data_deltas WHERE block_height >= ? AND block_height <= ? AND denom = ?", blockHeight-500000, blockHeight, denom).Scan(&annualizedRewards500kb)
	annualizedRewards500kb = annualizedRewards500kb * 365 * 24 * 60 * 60
	fmt.Sprintf("%.6f", annualizedRewards500kb)
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.6f", annualizedRewards500kb)

	return annualizedRewards500kb, nil
}
