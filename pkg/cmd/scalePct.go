/*
kubectl scale-pct deployment/<deployment-name> --percentage=10
https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/cmd/scale/scale.go
Need to get the current Percentage number
*/

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientV1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scale"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
)

var (
	namespaceExample = `
		# Scale up a Percentageet named 'foo' by 10%.
		%[1]s scale-pct --pct=10 rs/foo
		# Scale down a Percentageet named 'foo' by 10%.
		%[1]s scale-pct --pct=-10 rs/foo
	`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

// ScalePctOptions provides information required to update
// the current context on a user's KUBECONFIG
type ScalePctOptions struct {
	configFlags *genericclioptions.ConfigFlags
	PrintObj    printers.ResourcePrinterFunc
	PrintFlags  *genericclioptions.PrintFlags

	namespace        string
	enforceNamespace bool
	Percentage       int
	All              bool

	scaler    scale.Scaler
	builder   *resource.Builder
	clientSet kubernetes.Interface

	rawConfig api.Config
	args      []string

	genericclioptions.IOStreams
}

// NewScalePctOptions provides an instance of ScalePctOptions with default values
func NewScalePctOptions(streams genericclioptions.IOStreams) *ScalePctOptions {
	return &ScalePctOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		PrintFlags:  genericclioptions.NewPrintFlags("scaled"),

		IOStreams: streams,
	}
}

// NewCmdScalePct provides a cobra command wrapping ScalePctOptions
func NewCmdScalePct(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewScalePctOptions(streams)

	validArgs := []string{"deployment"}

	cmd := &cobra.Command{
		Use:     "ns [new-namespace] [flags]",
		Short:   "View or set the current namespace",
		Example: fmt.Sprintf(namespaceExample, "kubectl"),
		Run: func(c *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, c, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		ValidArgs: validArgs,
	}

	cmd.Flags().IntVar(&o.Percentage, "pct", o.Percentage, "The new desired number of Percentage. Required.")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required for updating the current context
func (o *ScalePctOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.args = args

	var err error

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = printer.PrintObj

	o.builder = f.NewBuilder()
	o.scaler, err = scaler(f)
	if err != nil {
		return err
	}

	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	o.namespace, o.enforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	o.clientSet, err = f.KubernetesClientSet()
	o.args = args
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

	if o.Percentage > 100 || o.Percentage < -100 {
		return fmt.Errorf("Percentage (pct) need to be in [-100, 100] range")
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
		SingleResourceType().
		Flatten().
		Do()
	err := r.Err()
	if err != nil {
		return err
	}

	r.Visit(func(info *resource.Info, err error) error {
		mapping := info.ResourceMapping()
		resources, _ := o.clientSet.AppsV1().Deployments(info.Namespace).Get(context.TODO(), info.Name, v1.GetOptions{})
		fmt.Println(info.Name)
		scaleReplicas := int(resources.Status.Replicas) + (int(resources.Status.Replicas) * o.Percentage / 100)
		if err := o.scaler.Scale(info.Namespace, info.Name, uint(scaleReplicas), &scale.ScalePrecondition{Size: -1}, &scale.RetryParams{}, &scale.RetryParams{}, mapping.Resource, false); err != nil {
			fmt.Println(err)
			return err
		}
		return o.PrintObj(info.Object, o.Out)
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

func getCurrentReplicas(f clientV1.AppsV1Client) {
}
