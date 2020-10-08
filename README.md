# ci-firewall

Used to run CI behind firewalls and collect result

## Pre-requisites

1. A publically accessible AMQP Message Queue like RabbitMQ. This should have a queue set aside to send the build requests
2. A public facing CI system or any place to download and run the requestor, which will request a build and recieve the result. It should be setup appropriately (see below)
3. A jenkins (behind your firewall) with rabbitmq-build-trigger [plugin](https://plugins.jenkins.io/rabbitmq-build-trigger/). The plugin should be configured to listen on the send queue, which should already exist on the server.
4. A Jenkins job/project which downloads the worker, and runs it with the appropriate parameters (see below). The job should be configured with a set of parameters.

### Requestor Configuration

The requestor MUST have following information in it, so that it can be passed as parameters to requestor cli (explained below)

- *AMQP URI*: The full URL of amqp server, including username/password and virtual servers if any
- *Send Queue Name*: The name of the send queue. This value should match what you configure on jenkins side
- *Recieve Queue Name(optional)*: The name of the recieve queue. Ideally, should be seperate from send queue, and ideally unique for each run request (latter is not compulsory, but will likely result in slowdown). By default, this will be taken as `rcv_jobname_kind_target`
- *Jenkins Job/Project*: The name of the jenkins job or project
- *Jenkins Token*: The token, as set on jenkins side for triggering the build.
- *Repo URL*: The URL of the repo to test
- *Target*: The target of the repo to test. Can be pr no, branch name or tag name
- *Kind*: The kind of target. `PR|BRANCH|TAG`
- *Run Script*: The script to run on the jenkins. Relative to repo root
- *Setup Script*: The script to run before run script. Relative to repo root.

### Worker Jenkins job configuration

The worker jenkins job MUST have following parameters defined. They do not have to be set, but configured.

- `REPO_URL`: The repo to test against
- `KIND`: The kind of build request
- `TARGET`: The target in the repo to test against. Example PR no/Branch Name etc
- `RUN_SCRIPT`: The entrypoint shell script to execute on worker side. Must handle the exit 1 case and be relative to repo root.
- `RCV_QUEUE_NAME`: Name of the recieve queue on the worker replies to requestor
- `SETUP_SCRIPT`: Name of setup script to run before running tests on worker side, Must handle exit 1 case and be relative to repo root

Apart from these core parameters, which will be sent by requestor, the following information will be needed in the worker. They will need to be passed to the worker cli as parameters in your jenkins (explained further down):

- *Jenkins URL*: The URL of the jenkins server (this should be already exposed and `JENKINS_URL` env in jenkins build)
- *Jenkins Job/Project*: The name of the jenkins job or project (This should already be exposed as `JOB_NAME` in jenkins build)
- *Jenkins Build Number*: The number of the jenkins build (this should already be exposed as `BUILD_NUMBER` in jenkins build).

- *AMQP URI*: The full URL of amqp server, including username/password and virtual servers, if any. looks for `AMQP_URI` env if set or can be passed as argument to cli.
- *Jenkins Robot User Name*: The name of the robot account to log into jenkins with. The user MUST be able to cancel builds for the given project. Looks for `JENKINS_ROBOT_USER` if set or can be passed as argument
- *Jenkins Robot User Password*: The password of above user. Looks for `JENKINS_ROBOT_PASSWORD` env, or can be passed as argument to cli
- *MultiNode*: If set to true, must have a test node db (see multinode testing below). Can be passed to cli

**NOTE**: When the worker is run, as below, it will ensure that 2 environment variables `BASE_OS=linux|windows|macos` and ARCH=`amd64|aarch64|etc` are available to your setup and run scripts, alongwith any other envs you pass to it.

### Build Script

Here is an example of a jenkins build script

```bash
WORKDIR="${KIND}_${TARGET}"
rm -rf $WORKDIR

mkdir -p $WORKDIR/{bin,repo}
PATH="$PATH:`pwd`/$WORKDIR/bin"
cd $WORKDIR
git clone https://github.com/mohammedzee1000/ci-firewall.git cif
cd cif
git checkout main
go build -mod=vendor cmd/ci-firewall/ci-firewall.go
cp -avrf ./ci-firewall ../bin/
cd ..
script --return -c "ci-firewall work --env 'FOO1=BAR1' --env 'FOO2=BAR2'" /dev/null
```

## Using the cli

### Requesting a build

Main Command:

```bash
 $ ci-firewall request [params]
 the result
```

#### Parameters

- *AMQP URL*: The full URL of amqp server. (env `AMQP_URI` or param `--amqpurl`)
- *Send Queue Name(optional)*: The name of the send queue. Defaults to `CI_SEND`. (param `--sendqueue`)
- *Recieve Queue Name(optional)*: The name of the queue in which replies are recieved. Defaults to `rcv_jenkinsproject_kind_target`. (param `recievequeue`)
- *Jenkins Project*: The name of jenkins project/job. (env `JOB_NAME` or param `--jenkinsproject`)
- *Jenkins Token*: The shared token for the jenkins project. (env `JOB_TOKEN` or param `--jenkinstoken`)
- *Repo URL*: The cloneable repo url. (env `REPO_URL` or param `repourl`)
- *Kind*: The kind of request, can be `PR|BRANCH|TAG`. (env `KIND` or param `--kind`)
- *Target*: The target repersent what pr/branch/tag needs to be checked out. (env `TARGET` or param `--target`)
- *Setup Script*: Script that runs before the test script, to do any setup needed. (env `SETUP_SCRIPT` or param `--setupscript`)
- *Run Script*: The test script to run. (env `RUN_SCRIPT` or param `--runscript`)
- *Timeout Duration(optional)*: The timeout duration for worker. Takes values like `1h10m10s`. Defaults to 12 minutes. (param `--timeout`)
