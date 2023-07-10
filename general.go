package main

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	minttypes "github.com/sge-network/sge/x/mint/types"
	tmrpc "github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"
)

func GeneralHandler(w http.ResponseWriter, r *http.Request, grpcConn *grpc.ClientConn, tmClient *tmrpc.HTTP) {
	requestStart := time.Now()

	sublogger := log.With().
		Str("request-id", uuid.New().String()).
		Logger()

	generalBondedTokensGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "cosmos_general_bonded_tokens",
			Help:        "Bonded tokens",
			ConstLabels: ConstLabels,
		},
	)

	generalNotBondedTokensGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "cosmos_general_not_bonded_tokens",
			Help:        "Not bonded tokens",
			ConstLabels: ConstLabels,
		},
	)

	generalCommunityPoolGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "cosmos_general_community_pool",
			Help:        "Community pool",
			ConstLabels: ConstLabels,
		},
		[]string{"denom"},
	)

	generalSupplyTotalGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "cosmos_general_supply_total",
			Help:        "Total supply",
			ConstLabels: ConstLabels,
		},
		[]string{"denom"},
	)

	generalInflationGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "sge_general_inflation",
			Help:        "Total supply",
			ConstLabels: ConstLabels,
		},
	)

	generalPhaseProvisions := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "sge_general_phase_provisions",
			Help:        "Phase provisions",
			ConstLabels: ConstLabels,
		},
		[]string{"denom"},
	)

	generalUpgradePlannedGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "cosmos_upgrade_planned",
			Help:        "Upgrade planned",
			ConstLabels: ConstLabels,
		},
		[]string{"name", "info"},
	)

	generalUpgradePlanHeightGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "cosmos_upgrade_plan_height",
			Help:        "Upgrade plan height",
			ConstLabels: ConstLabels,
		},
	)

	generalAvgBlockTimeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "cosmos_avg_block_time",
			Help:        "Average block time",
			ConstLabels: ConstLabels,
		},
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(generalBondedTokensGauge)
	registry.MustRegister(generalNotBondedTokensGauge)
	registry.MustRegister(generalCommunityPoolGauge)
	registry.MustRegister(generalSupplyTotalGauge)
	registry.MustRegister(generalInflationGauge)
	registry.MustRegister(generalPhaseProvisions)
	registry.MustRegister(generalUpgradePlannedGauge)
	registry.MustRegister(generalUpgradePlanHeightGauge)
	registry.MustRegister(generalAvgBlockTimeGauge)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying staking pool")
		queryStart := time.Now()

		stakingClient := stakingtypes.NewQueryClient(grpcConn)
		response, err := stakingClient.Pool(
			context.Background(),
			&stakingtypes.QueryPoolRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get staking pool")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying staking pool")

		generalBondedTokensGauge.Set(float64(response.Pool.BondedTokens.Int64()))
		generalNotBondedTokensGauge.Set(float64(response.Pool.NotBondedTokens.Int64()))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying distribution community pool")
		queryStart := time.Now()

		distributionClient := distributiontypes.NewQueryClient(grpcConn)
		response, err := distributionClient.CommunityPool(
			context.Background(),
			&distributiontypes.QueryCommunityPoolRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get distribution community pool")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying distribution community pool")

		for _, coin := range response.Pool {
			if value, err := strconv.ParseFloat(coin.Amount.String(), 64); err != nil {
				sublogger.Error().
					Err(err).
					Msg("Could not get community pool coin")
			} else {
				generalCommunityPoolGauge.With(prometheus.Labels{
					"denom": Denom,
				}).Set(value / DenomCoefficient)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying bank total supply")
		queryStart := time.Now()

		bankClient := banktypes.NewQueryClient(grpcConn)
		response, err := bankClient.TotalSupply(
			context.Background(),
			&banktypes.QueryTotalSupplyRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get bank total supply")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying bank total supply")

		for _, coin := range response.Supply {
			if value, err := strconv.ParseFloat(coin.Amount.String(), 64); err != nil {
				sublogger.Error().
					Err(err).
					Msg("Could not get total supply")
			} else {
				generalSupplyTotalGauge.With(prometheus.Labels{
					"denom": Denom,
				}).Set(value / DenomCoefficient)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying inflation")
		queryStart := time.Now()

		mintClient := minttypes.NewQueryClient(grpcConn)
		response, err := mintClient.Inflation(
			context.Background(),
			&minttypes.QueryInflationRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get inflation")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying inflation")

		if value, err := strconv.ParseFloat(response.Inflation.String(), 64); err != nil {
			sublogger.Error().
				Err(err).
				Msg("Could not get inflation")
		} else {
			generalInflationGauge.Set(value)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying phase provisions")
		queryStart := time.Now()

		mintClient := minttypes.NewQueryClient(grpcConn)
		response, err := mintClient.PhaseProvisions(
			context.Background(),
			&minttypes.QueryPhaseProvisionsRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get phase provisions")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying phase provisions")

		if value, err := strconv.ParseFloat(response.PhaseProvisions.String(), 64); err != nil {
			sublogger.Error().
				Err(err).
				Msg("Could not get phase provisions")
		} else {
			generalPhaseProvisions.With(prometheus.Labels{
				"denom": Denom,
			}).Set(value / DenomCoefficient)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started querying upgrade plan")
		queryStart := time.Now()

		upgradeClient := upgradetypes.NewQueryClient(grpcConn)
		response, err := upgradeClient.CurrentPlan(
			context.Background(),
			&upgradetypes.QueryCurrentPlanRequest{},
		)
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get upgrade plan")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying upgrade plan")

		if response.Plan != nil {
			generalUpgradePlannedGauge.With(
				prometheus.Labels{"name": response.Plan.Name, "info": response.Plan.Info},
			).Set(float64(1))
			generalUpgradePlanHeightGauge.Set(float64(response.Plan.Height))
		} else {
			sublogger.Debug().Msg("No Upgrade planned")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sublogger.Debug().Msg("Started average block time")
		queryStart := time.Now()

		response, err := tmClient.Status(context.Background())
		if err != nil {
			sublogger.Error().Err(err).Msg("Could not get average block time")
			return
		}

		sublogger.Debug().
			Float64("request-time", time.Since(queryStart).Seconds()).
			Msg("Finished querying average block time")

		timeDiff := response.SyncInfo.LatestBlockTime.Sub(refBlockTime).Seconds()
		blockDiff := response.SyncInfo.LatestBlockHeight - refBlockHeight

		avgBlockTime := timeDiff / float64(blockDiff)

		generalAvgBlockTimeGauge.Set(avgBlockTime)
	}()

	wg.Wait()

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
	sublogger.Info().
		Str("method", "GET").
		Str("endpoint", "/metrics/general").
		Float64("request-time", time.Since(requestStart).Seconds()).
		Msg("Request processed")
}
