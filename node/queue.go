package node

import (
	"github.com/virtual-kubelet/virtual-kubelet/internal/queue"
)

// These are exportable definitions of the queue package:

// ShouldRetryFunc is a mechanism to have a custom retry policy
//
// it is passed metadata about the work item when the handler returns an error. It returns the following:
// * The key
// * The number of attempts that this item has already had (and failed)
// * The (potentially wrapped) error from the queue handler.
//
// The return value is an error, and optionally an amount to delay the work.
// If an error is returned, the work will be aborted, and the returned error is bubbled up. It can be the error that
// was passed in or that error can be wrapped.
//
// If the work item should be is to be retried, a delay duration may be specified. The delay is used to schedule when
// the item should begin processing relative to now, it does not necessarily dictate when the item will start work.
// Items are processed in the order they are scheduled. If the delay is nil, it will fall  back to the default behaviour
// of the queue, and use the rate limiter that's configured to determine when to start work.
//
// If the delay is negative, the item will be scheduled "earlier" than now. This will result in the item being executed
// earlier than other items in the FIFO work order.
type ShouldRetryFunc = queue.ShouldRetryFunc

// DefaultRetryFunc is the default function used for retries by the queue subsystem. Its only policy is that it gives up
// after MaxRetries, and falls back to the rate limiter for all other retries.
var DefaultRetryFunc = queue.DefaultRetryFunc

// MaxRetries is the number of times we try to process a given key before permanently forgetting it.
var MaxRetries = queue.MaxRetries
