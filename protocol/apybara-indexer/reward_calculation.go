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
	rewardDataDeltaWithAnnualizeRewards, rewardDataDeltaSumOfDelta, err := r.AnnualizedRewards(ctx.BlockHeight(), denom)
	if err != nil {
		fmt.Println("error calculating annualizedRewards500kb", err)
		return 0, err
	}
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.18f", rewardDataDeltaWithAnnualizeRewards)
	rewardDataDelta.SumDelta500kb = fmt.Sprintf("%.18f", rewardDataDeltaSumOfDelta)
	database.Save(&rewardDataDelta)
	return rewardDelta, nil
}

func (r RewardCalculatorService) AnnualizedRewards(blockHeight int64, denom string) (float64, float64, error) {
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	db := r.Database
	var rewardDataDelta RewardDataDelta
	var annualizedRewards500kb float64
	var sumOfDeltaPast500kBlocks float64
	var timestamp500kBlocks int64
	db.Model(&rewardDataDelta).Where("block_height = ? AND denom = ?", blockHeight, denom).First(&rewardDataDelta)
	//SELECT SUM(cast(delta AS DOUBLE PRECISION)) FROM reward_data_delta WHERE block_height < 2860685 - 1 AND block_height >= 2860685 - 500000 and denom = 'adydx';
	//define annualizedRewards500kb = {sum of rewardDelta over the past 500k blocks} / (timestamp[n] - timestamp[n-500k blocks]) * 365 * 24 * 60 * 60
	//	SELECT
	//	(SELECT timestamp
	//	FROM reward_data_delta
	//	WHERE block_height = 2887336
	//	AND denom = 'adydx'
	//	) -
	//		COALESCE(
	//			(SELECT timestamp
	//	FROM reward_data_delta
	//	WHERE block_height = 2887336 - 500000
	//	AND denom = 'adydx'
	//	ORDER BY timestamp DESC
	//	LIMIT 1),
	//	(SELECT timestamp
	//	FROM reward_data_delta
	//	WHERE denom = 'adydx'
	//	ORDER BY timestamp ASC
	//	LIMIT 1)
	//) AS time_difference

	// sum delta over the past 3 days
	var sumOfDeltaPast3Days float64
	timestampNow := rewardDataDelta.Timestamp
	timestamp3DaysAgo := timestampNow - 3*24*60*60

	//// sum delta within the past 3 days
	db.Raw("select sum(cast(delta as double precision)) from reward_data_delta where timestamp >= ? and timestamp <= ? and denom = ?", timestamp3DaysAgo, timestampNow, denom).Scan(&sumOfDeltaPast3Days)

	// time difference between now and past 500k blocks
	db.Raw("SELECT (SELECT timestamp FROM reward_data_delta WHERE block_height = ? AND denom = ?) - COALESCE((SELECT timestamp FROM reward_data_delta WHERE block_height = ? - 500000 AND denom = ? ORDER BY timestamp DESC LIMIT 1), (SELECT timestamp FROM reward_data_delta WHERE denom = ? ORDER BY timestamp ASC LIMIT 1)) AS time_difference", blockHeight, denom, blockHeight, denom, denom).Scan(&timestamp500kBlocks)

	// sum delta over the past 500k blocks
	db.Raw("SELECT SUM(cast(delta AS DOUBLE PRECISION)) FROM reward_data_delta WHERE block_height < ? - 1 AND block_height >= ? - 500000 and denom = ?", blockHeight, blockHeight, denom).Scan(&sumOfDeltaPast500kBlocks)
	annualizedRewards500kb = sumOfDeltaPast500kBlocks / float64(timestamp500kBlocks) * 365 * 24 * 60 * 60
	fmt.Sprintf("%.6f", annualizedRewards500kb)
	rewardDataDelta.AnnualizeReward500kb = fmt.Sprintf("%.18f", annualizedRewards500kb)
	rewardDataDelta.SumDelta500kb = fmt.Sprintf("%.18f", sumOfDeltaPast500kBlocks)
	rewardDataDelta.SumDeltaPast3Days = fmt.Sprintf("%.18f", sumOfDeltaPast3Days)

	return annualizedRewards500kb, sumOfDeltaPast500kBlocks, nil
}
