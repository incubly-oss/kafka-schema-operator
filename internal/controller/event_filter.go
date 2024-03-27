package controller

import (
	"fmt"
	"math"
	"time"

	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Status struct {
	LastRetryTs int64
	RetryCount  int
}

const minInterval = 50 * time.Millisecond
const maxInterval = 30 * time.Minute

func ignoreIfBeforeRequeueDelay(
	objToStatus func(client.Object) (Status, error),
	requeueDelay time.Duration) predicate.Predicate {

	check := func(obj client.Object) bool {
		logger := log.FromContext(context.TODO())
		fullName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
		logger.V(4).Info(fmt.Sprintf("Checking %s for reconciliation", fullName))

		status, err := objToStatus(obj)
		if err != nil {
			logger.Info(fmt.Sprintf("Failed to extract status for resource %s. Trying to reconcile", fullName))
			return true
		}
		nextTickInThePast := isNextTickInThePast(status, requeueDelay)
		if nextTickInThePast {
			logger.V(4).Info(fmt.Sprintf("Attempting reconciliation of %s for the %d-th time",
				fullName, status.RetryCount))
		} else {
			logger.V(4).Info(fmt.Sprintf("Skipping reconciliation of %s for now", fullName))
		}
		return nextTickInThePast
	}

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return check(e.ObjectOld)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return check(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return check(e.Object)
		},
	}
}

func isNextTickInThePast(status Status, delay time.Duration) bool {
	nextTickMillis := calculateNextTick(status, delay)
	return nextTickMillis < time.Now().UnixMilli()
}

func calculateNextTick(status Status, delay time.Duration) int64 {
	delayMillis := delay.Milliseconds()
	if delayMillis < 0 {
		// TODO: previously that ment - "don't reconcile", but only for successful results. What do we do?
		if status.RetryCount == 0 {
			// very temporary: assuming it was successful (retry=0), hack to tell "don't reconcile"
			return time.Now().UnixMilli() + time.Hour.Milliseconds()
		} else {
			return expBackoffDelay(status)
		}
	} else if delayMillis == 0 {
		return expBackoffDelay(status)
	} else {
		return status.LastRetryTs + delayMillis
	}
}

func expBackoffDelay(status Status) int64 {
	expBackoff := minInterval.Milliseconds() * int64(math.Pow(2, float64(status.RetryCount)))
	if expBackoff > maxInterval.Milliseconds() {
		expBackoff = maxInterval.Milliseconds()
	}
	return status.LastRetryTs + expBackoff
}
