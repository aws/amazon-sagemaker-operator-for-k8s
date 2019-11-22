### SMLogs Kubectl Plugin

This Go module implements a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) for accessing cloudwatch logs of SageMaker jobs managed by the SageMaker Kubernetes operator.

### Installation
The [official installation guide on kubectl plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/#installing-kubectl-plugins) is helpful. To be able to run the plugin via `kubectl`, you need to
place the binary on your PATH. You can either add the `./bin` directory to your path or symlink the binary into your path.

```bash
# Generate the binary by running below command
make build 

# Above command will create a binary called kubectl-smlogs in `bin` directory of your current working directory.

# Symlink assumming ~/bin exists and is on your PATH:
ln -s "$(pwd)"/bin/kubectl-smlogs ~/bin

# PATH
export PATH=$PATH:"$(pwd)"/bin
```

### Authentication
The plugin uses your local aws config. You can create an aws config [using the official AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html).

### Example Usage
#### 1. Using Help
 
Cobra generates help and usage hints based on the source which you can utilize as:
`kubectl smlogs --help`

This will list the command, subcommands and flags available for use. 

#### 2. Logs for a Training job 
`kubectl smlogs TrainingJob <training-job-name> -f`

or even the following works - 
`kubectl smlogs trainingjobs <training-job-name>`


#### 3. Logs for a HPO job 
 - There exists a subcommand for HPO but only to guide the user to use the right subcommand as: 
```
k smlogs HyperParameterTuningJob <hpo-job-name>
Error: For HPO logs, Refer to the the Spawned Training Jobs. Use `kubectl get TrainingJob` to list resource names
```
 
 - Use the following command to get a list of the HPO spawned training jobs. 
`kubectl get trainingjob`

 - and now get the logs for any one of these
`kubectl smlogs TrainingJob <hpo-spawned-training-job-name>`


#### 4. Logs for a BatchTransform job 
`kubectl smlogs BatchTransformJob <transform-job-name> -f`

or even the following works - 
`kubectl smlogs batchtransformjobs <transform-job-name>`
