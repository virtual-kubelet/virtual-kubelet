// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package redshift

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
)

// WaitUntilClusterAvailable uses the Amazon Redshift API operation
// DescribeClusters to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *Redshift) WaitUntilClusterAvailable(input *DescribeClustersInput) error {
	return c.WaitUntilClusterAvailableWithContext(aws.BackgroundContext(), input)
}

// WaitUntilClusterAvailableWithContext is an extended version of WaitUntilClusterAvailable.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *Redshift) WaitUntilClusterAvailableWithContext(ctx aws.Context, input *DescribeClustersInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilClusterAvailable",
		MaxAttempts: 30,
		Delay:       request.ConstantWaiterDelay(60 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "Clusters[].ClusterStatus",
				Expected: "available",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Clusters[].ClusterStatus",
				Expected: "deleting",
			},
			{
				State:    request.RetryWaiterState,
				Matcher:  request.ErrorWaiterMatch,
				Expected: "ClusterNotFound",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *DescribeClustersInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeClustersRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// WaitUntilClusterDeleted uses the Amazon Redshift API operation
// DescribeClusters to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *Redshift) WaitUntilClusterDeleted(input *DescribeClustersInput) error {
	return c.WaitUntilClusterDeletedWithContext(aws.BackgroundContext(), input)
}

// WaitUntilClusterDeletedWithContext is an extended version of WaitUntilClusterDeleted.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *Redshift) WaitUntilClusterDeletedWithContext(ctx aws.Context, input *DescribeClustersInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilClusterDeleted",
		MaxAttempts: 30,
		Delay:       request.ConstantWaiterDelay(60 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:    request.SuccessWaiterState,
				Matcher:  request.ErrorWaiterMatch,
				Expected: "ClusterNotFound",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Clusters[].ClusterStatus",
				Expected: "creating",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Clusters[].ClusterStatus",
				Expected: "modifying",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *DescribeClustersInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeClustersRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// WaitUntilClusterRestored uses the Amazon Redshift API operation
// DescribeClusters to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *Redshift) WaitUntilClusterRestored(input *DescribeClustersInput) error {
	return c.WaitUntilClusterRestoredWithContext(aws.BackgroundContext(), input)
}

// WaitUntilClusterRestoredWithContext is an extended version of WaitUntilClusterRestored.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *Redshift) WaitUntilClusterRestoredWithContext(ctx aws.Context, input *DescribeClustersInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilClusterRestored",
		MaxAttempts: 30,
		Delay:       request.ConstantWaiterDelay(60 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "Clusters[].RestoreStatus.Status",
				Expected: "completed",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Clusters[].ClusterStatus",
				Expected: "deleting",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *DescribeClustersInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeClustersRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// WaitUntilSnapshotAvailable uses the Amazon Redshift API operation
// DescribeClusterSnapshots to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *Redshift) WaitUntilSnapshotAvailable(input *DescribeClusterSnapshotsInput) error {
	return c.WaitUntilSnapshotAvailableWithContext(aws.BackgroundContext(), input)
}

// WaitUntilSnapshotAvailableWithContext is an extended version of WaitUntilSnapshotAvailable.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *Redshift) WaitUntilSnapshotAvailableWithContext(ctx aws.Context, input *DescribeClusterSnapshotsInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilSnapshotAvailable",
		MaxAttempts: 20,
		Delay:       request.ConstantWaiterDelay(15 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "Snapshots[].Status",
				Expected: "available",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Snapshots[].Status",
				Expected: "failed",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Snapshots[].Status",
				Expected: "deleted",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *DescribeClusterSnapshotsInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeClusterSnapshotsRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}
