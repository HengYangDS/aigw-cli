package selection

import (
	"fmt"
	"strconv"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

type retryOptions struct {
	request, stream uint64
	native          bool
}

func (options *retryOptions) bind(cmd *cobra.Command, admission domain.Admission) {
	clients := retryBudgetClients(admission)
	cmd.Flags().Uint64Var(&options.request, "request-max-retries", 0, "Explicit native request retry budget for this "+clients+" connection; zero disables client request retries")
	cmd.Flags().Uint64Var(&options.stream, "stream-max-retries", 0, "Explicit native stream retry budget for this "+clients+" connection; zero disables client stream retries")
	cmd.Flags().BoolVar(&options.native, "native-retries", false, "Remove this connection's retry policy and use client native defaults")
}

// retryBudgetClients names the clients that honor per-connection retry budgets.
func retryBudgetClients(admission domain.Admission) string {
	var labels []string
	for _, spec := range admission.Specs() {
		if spec.RetryBudgets {
			labels = append(labels, spec.Label)
		}
	}
	return strings.Join(labels, " or ")
}

func (options *retryOptions) connection(cmd *cobra.Command, forwarding *string) (domain.ConnectionChange, error) {
	change := domain.ConnectionChange{ForwardingEndpoint: forwarding, NativeRetries: options.native}
	request, stream := cmd.Flags().Changed("request-max-retries"), cmd.Flags().Changed("stream-max-retries")
	if options.native && (request || stream) {
		return change, fmt.Errorf("--native-retries and explicit retry budgets are mutually exclusive")
	}
	if request || stream {
		change.RetryPolicy = &domain.RetryPolicy{}
		if request {
			change.RetryPolicy.RequestMaxRetries = &options.request
		}
		if stream {
			change.RetryPolicy.StreamMaxRetries = &options.stream
		}
	}
	return change, nil
}

func retryFields(policy *domain.RetryPolicy) []presentation.Field {
	if policy == nil {
		return []presentation.Field{{Label: "Retries", Value: "Client native defaults"}}
	}
	var fields []presentation.Field
	if policy.RequestMaxRetries != nil {
		fields = append(fields, presentation.Field{Label: "Request retries", Value: strconv.FormatUint(*policy.RequestMaxRetries, 10)})
	}
	if policy.StreamMaxRetries != nil {
		fields = append(fields, presentation.Field{Label: "Stream retries", Value: strconv.FormatUint(*policy.StreamMaxRetries, 10)})
	}
	return fields
}
