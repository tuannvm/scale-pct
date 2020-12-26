/*
kubectl scale-pct deployment/<deployment-name> --percentage=10
https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/cmd/scale/scale.go
Need to get the current replicas number
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scale"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

var (
	namespaceExample = `
	# view the current namespace in your KUBECONFIG
	%[1]s ns

	# view all of the namespaces in use by contexts in your KUBECONFIG
	%[1]s ns --list

	# switch your current-context to one that contains the desired namespace
	%[1]s ns foo
`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

// ScalePctOptions provides information required to update
// the current context on a user's KUBECONFIG
type ScalePctOptions struct {
	configFlags *genericclioptions.ConfigFlags

	namespace        string
	enforceNamespace bool
	Replicas         int
	All              bool

	scaler  scale.Scaler
	builder *resource.Builder

	rawConfig api.Config
	args      []string

	genericclioptions.IOStreams
}

// NewScalePctOptions provides an instance of ScalePctOptions with default values
func NewScalePctOptions(streams genericclioptions.IOStreams) *ScalePctOptions {
	return &ScalePctOptions{
		configFlags: genericclioptions.NewConfigFlags(true),

		IOStreams: streams,
	}
}

// NewCmdScalePct provides a cobra command wrapping ScalePctOptions
func NewCmdScalePct(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewScalePctOptions(streams)

	cmd := &cobra.Command{
		Use:          "ns [new-namespace] [flags]",
		Short:        "View or set the current namespace",
		Example:      fmt.Sprintf(namespaceExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(f, c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&o.Replicas, "replicas", o.Replicas, "The new desired number of replicas. Required.")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required for updating the current context
func (o *ScalePctOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.args = args

	var err error

	o.builder = f.NewBuilder()
	o.scaler, err = scaler(f)
	if err != nil {
		return err
	}

	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	o.namespace, o.enforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	return nil
}

// Validate ensures that all required arguments and flag values are provided
func (o *ScalePctOptions) Validate() error {
	if len(o.rawConfig.CurrentContext) == 0 {
		return errNoContext
	}
	if len(o.args) > 1 {
		return fmt.Errorf("either one or no arguments are allowed")
	}

	return nil
}

// Run lists all available namespaces on a user's KUBECONFIG or updates the
// current context based on a provided namespace.
func (o *ScalePctOptions) Run() error {

	r := o.builder.
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.namespace).
		DefaultNamespace().
		ResourceTypeOrNameArgs(o.All, o.args...).
		Flatten().
		Do()

	r.Visit(func(info *resource.Info, err error) error {
		fmt.Println(o.Replicas)
		mapping := info.ResourceMapping()
		if err := o.scaler.Scale(info.Namespace, info.Name, uint(o.Replicas), &scale.ScalePrecondition{Size: -1}, &scale.RetryParams{}, &scale.RetryParams{}, mapping.Resource, false); err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	})
	return nil
}

func scaler(f cmdutil.Factory) (scale.Scaler, error) {
	scalesGetter, err := cmdutil.ScaleClientFn(f)
	if err != nil {
		return nil, err
	}

	return scale.NewScaler(scalesGetter), nil
}
