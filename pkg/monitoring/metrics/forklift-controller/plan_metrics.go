package forklift_controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var activePlanStatuses = make(map[string]struct{})
var activePlanAlertStatuses = make(map[string]struct{})

// Calculate Plans metrics every 10 seconds
func RecordPlanMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all plans objects
			plans := api.PlanList{}
			err := c.List(context.TODO(), &plans)

			// if error occurs, retry 10 seconds later
			if err != nil {
				fmt.Printf("Metrics Plans list error: %v\n", err)
				continue
			}

			// Initialize or reset the counter map at the beginning of each iteration
			plansCounterMap := make(map[string]float64)
			planAlertsMap := make(map[string]struct{})

			for _, m := range plans.Items {
				sourceProvider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Source.Namespace, Name: m.Spec.Provider.Source.Name}, &sourceProvider)
				if err != nil {
					continue
				}

				destProvider := api.Provider{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Destination.Namespace, Name: m.Spec.Provider.Destination.Name}, &destProvider)
				if err != nil {
					continue
				}

				isLocal := destProvider.Spec.URL == ""

				var target, mode, key string
				if isLocal {
					target = Local
				} else {
					target = Remote
				}
				if m.IsWarm() {
					mode = Warm
				} else {
					mode = Cold
				}

				provider := sourceProvider.Type().String()
				planUID := string(m.UID)
				planName := m.GetName()
				phase := ""

				if m.Status.HasCondition(Succeeded) {
					key = fmt.Sprintf("%s|%s|%s|%s", Succeeded, provider, mode, target)
					plansCounterMap[key]++

					// If plan succeeded, create an alert metric
					phase = Completed
					alertKey := fmt.Sprintf("%s|%s|%s|%s", key, planUID, planName, phase)
					activePlanAlertStatuses[alertKey] = struct{}{}
					planAlertsMap[alertKey] = struct{}{}
				}
				if m.Status.HasCondition(Failed) {
					key = fmt.Sprintf("%s|%s|%s|%s", Failed, provider, mode, target)
					plansCounterMap[key]++

					// If plan failed, create an alert metric
					for _, vm := range m.Status.Migration.VMs {
						if vm.Error != nil {
							phase = fmt.Sprintf("%s,%s", phase, vm.Error.Phase)
						}
					}
					phase = strings.Trim(phase, ",")
					alertKey := fmt.Sprintf("%s|%s|%s|%s", key, planUID, planName, phase)
					activePlanAlertStatuses[alertKey] = struct{}{}
					planAlertsMap[alertKey] = struct{}{}
				}
				if m.Status.HasCondition(Executing) {
					key = fmt.Sprintf("%s|%s|%s|%s", Executing, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Running) {
					key = fmt.Sprintf("%s|%s|%s|%s", Running, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Pending) {
					key = fmt.Sprintf("%s|%s|%s|%s", Pending, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Canceled) {
					key = fmt.Sprintf("%s|%s|%s|%s", Canceled, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Blocked) {
					key = fmt.Sprintf("%s|%s|%s|%s", Blocked, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Deleted) {
					key = fmt.Sprintf("%s|%s|%s|%s", Deleted, provider, mode, target)
					plansCounterMap[key]++
				}
			}

			for key, value := range plansCounterMap {
				parts := strings.Split(key, "|")
				planStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3]}).Set(value)
				activePlanStatuses[key] = struct{}{}
			}

			for planStatus := range activePlanStatuses {
				if _, exists := plansCounterMap[planStatus]; !exists {
					parts := strings.Split(planStatus, "|")
					planStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3]}).Set(0)
					delete(activePlanStatuses, planStatus)
				}
			}

			for key := range planAlertsMap {
				parts := strings.Split(key, "|")
				planAlertStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3], "plan": parts[4], "plan_name": parts[5], "phase": parts[6]}).Set(1)
				activePlanAlertStatuses[key] = struct{}{}
			}

			for planAlertStatus := range activePlanAlertStatuses {
				if _, exists := planAlertsMap[planAlertStatus]; !exists {
					parts := strings.Split(planAlertStatus, "|")
					planAlertStatusGauge.DeleteLabelValues(parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6])
					delete(activePlanAlertStatuses, planAlertStatus)
				}
			}
		}
	}()
}
