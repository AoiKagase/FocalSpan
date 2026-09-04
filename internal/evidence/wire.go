package evidence

import (
	"encoding/json"
	"fmt"

	"github.com/focalspan/focalspan/internal/budget"
)

func Summary(packet Packet) string {
	return fmt.Sprintf("items=%d tokens=%d/%d omitted=%d", len(packet.Evidence), packet.Budget.Used, packet.Budget.Limit, packet.Budget.Omitted)
}

func MeasureModelVisible(packet Packet, estimator budget.TokenEstimator) int {
	if estimator == nil {
		estimator = budget.NewEstimator()
	}
	payload, err := json.Marshal(packet)
	if err != nil {
		return budget.MaxBudget + 1
	}
	return estimator.Estimate(string(payload) + "\n" + Summary(packet))
}

func settleWireUsage(packet *Packet, estimator budget.TokenEstimator) int {
	for iteration := 0; iteration < 4; iteration++ {
		measured := MeasureModelVisible(*packet, estimator)
		if measured == packet.Budget.Used {
			return measured
		}
		packet.Budget.Used = measured
	}
	measured := MeasureModelVisible(*packet, estimator)
	packet.Budget.Used = measured
	return MeasureModelVisible(*packet, estimator)
}
