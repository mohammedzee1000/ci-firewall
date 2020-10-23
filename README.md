# ci-firewall

Used to run CI behind firewalls and collect result stream it to a public place

## DISCLAIMER

**WARNING**: By using this, you are punching a hole through your firewall. Please ensure that your trigger mechanism is configured correctly so as to not allow random people ot change you code/scripts (like an ok-to-test filter for eg) in a way to abuse your system(by triggering random builds etc) or expose to private information.

**WARNING**: This tool does NOT HANDLE scrubbing of logs to prevent leakage of private information. You are respondsible for your own scrubbing. Whatever your script prints to stdout/stderr, this tool streams. So ensure your script is handles the scrubbing

## Pre-requisites

1. A publically accessible AMQP RabbitMQ (with JMS plugin). This should have a `queue (durable=true,autodelete=false)`, alongwith an `fanout type exchange` bound to the said queue, with a `topic(read routing key)` set aside to send the build requests.
2. A public facing CI system, which preferably allows for secrets for certain secrets (such amqp URI, kind, target, repourl etc) or any place to download and run the requestor, which will request a build and recieve the result. The requestor should be setup correctly. (see below)
3. A jenkins (behind your firewall) with jms plugin [plugin](https://plugins.jenkins.io/jms-messaging/). The plugin should be configured to use `RabbitMQ Provider` and to listen on the send queue, using the exchange and topic, which should already exist on the server(pre-created see 1).
4. A Jenkins job/project which downloads the worker, and runs it with the appropriate parameters (see below). The job should be configured with a set of parameters.

### Requestor Configuration

The requestor MUST have following information in it, so that it can be passed as parameters to requestor cli (explained below)

- *AMQP URI*: The full URL of amqp server, including username/password and virtual servers if any
- *Send Queue Name*: The name of the send queue. This value should match what you configure on jenkins side
- *Recieve Queue Name(optional)*: The name of the recieve queue. Ideally, should be seperate from send queue, and ideally unique for each run request (latter is not compulsory, but will likely result in slowdown). By default, this will be taken as `amqp_jenkinsjob_kind_target`
- *Jenkins Job/Project*: The name of the jenkins job or project
- *Repo URL*: The URL of the repo to test
- *Target*: The target of the repo to test. Can be pr no, branch name or tag name
- *Kind*: The kind of target. `PR|BRANCH|TAG`
- *Run Script*: The script to run on the jenkins. Relative to repo root
- *Setup Script*: The script to run before run script. Relative to repo root.
- *Run Script URL*: The url to remote run script, if any. Will be saved as `Run Script`

### Worker Jenkins job configuration

The following information will be needed in the worker. They will need to be passed to the worker cli as parameters in your jenkins (explained further down):

- *Jenkins URL*: The URL of the jenkins server (this should be already exposed and `JENKINS_URL` env in jenkins build)
- *Jenkins Job/Project*: The name of the jenkins job or project (This should already be exposed as `JOB_NAME` in jenkins build)
- *Jenkins Build Number*: The number of the jenkins build (this should already be exposed as `BUILD_NUMBER` in jenkins build).

- *AMQP URI*: The full URL of amqp server, including username/password and virtual servers.  `AMQP_URI` env if set or can be passed as argument to cli.
- *Jenkins Robot User Name*: The name of the robot account to log into jenkins with. The user MUST be able to cancel builds for the given project. Looks for `JENKINS_ROBOT_USER` if set or can be passed as argument
- *Jenkins Robot User Password*: The password of above user. Looks for `JENKINS_ROBOT_PASSWORD` env, or can be passed as argument to cli
- *SSH Node file(optional)*: If provided, repersents path to json file containing infor needed to ssh into nodes and run tests. Can be passed to cli (see below for more information)
- *CI Message Variable* The variable containing, as configured in your CI Event Subscriber (defaults to `CI)MESSAGE` env)

It is also a good idea to ensure the jenkins job cleans up after itself by enabling `Delete workspace before build starts` and maybe even `Execute concurrent builds if necessary` depending on if your are ok with each build running only after previous build is finished.

Any other parameter definitions are on you.

#### Jenkins Build Script

Here is an example of a jenkins build script

```bash
rm -rf ./*
curl -kJLO https://github.com/mohammedzee1000/ci-firewall/releases/download/${CI_FIREWALL_VERSION}/ci-firewall.tar.gz
tar -xzf ./ci-firewall.tar.gz && rm -rf ./ci-firewall.tar.gz && chmod +x ./ci-firewall
./ci-firewall work --env 'FOO1=BAR1' --env 'FOO2=BAR2'
rm -rf ./*
```

**WARNING**: It is absolutely nessasary to check if your jenkins runs tasks with a psedu terminal, and if not,  run the worker inside a `script` as `script --return -c "./ci-firewall work --env 'FOO1=BAR1' --env 'FOO2=BAR2'" /dev/null` so that it gets a pseudo terminal. Without a pseudoterminal, it can cause some of the operations to fail !!

**NOTE**: When the worker is run, it will ensure that 2 environment variables `BASE_OS=linux|windows|mac` and ARCH=`amd64|aarch64|etc` are available to your setup and run scripts, alongwith any other envs you pass to it. *For now, we are assuming the jenkins slave is always linux and amd64.* If sshnode is provided then it can vary based on what is defined there.

## Using the cli

### Requesting a build

Main Command:

```bash
 $ ci-firewall request [params]
 the result
```

#### Request Parameters

- *AMQP URL*: The full URL of amqp server. (env `AMQP_URI` or param `--amqpurl`)
- *Send Queue Name(optional)*: The name of the send queue. Defaults to `amqp.ci.queue.send`. (param `--sendqueue`). This is the same queue that your jenkins is subscribed to.
- *Send Exchange*: The name of the send queue exchange. If it already exists, it should be of kind fanout. Defaults to `amqp.ci.exchange.send` (param `--sendexchange`)
- *Send Topic*: This is the topic binding between the send queue and send exchange. Defaults to `amqp.ci.topic.send` (param `--sendtopic`)
- *Recieve Ident(optional)*: The name of the queue in which replies are recieved. Defaults to `amqp.ci.rcv.jenkinsproject.kind.target`. (param `recievequeue`)
- *Repo URL*: The cloneable repo url. (env `REPO_URL` or param `repourl`)
- *Jenkins Job/Project*: The name of jenkins project/job. (env `JOB_NAME` or param `--jenkinsproject`).
- *Kind*: The kind of request, can be `PR|BRANCH|TAG`. Defaults to `PR`. (env `KIND` or param `--kind`)
- *Target*: The target repersent what pr/branch/tag needs to be checked out. (env `TARGET` or param `--target`)
- *Setup Script(optional)*: Script that runs before the test script, to do any setup needed. (env `SETUP_SCRIPT` or param `--setupscript`)
- *Run Script*: The test script to run. (env `RUN_SCRIPT` or param `--runscript`)
- *Timeout Duration(optional)*: The timeout duration for worker. Takes values like `1h10m10s`. Defaults to 15 minutes. (param `--timeout`)
- *Run Script URL(optional)*: The URL to the runscript if any. This will be saved as `Run Script`. (param `--runscripturl`)

### Working on a build

```bash
 $ ci-firewall work [params]
 the result
```

#### Work Parameters

- *AMQP URL*: The full URL of amqp server. (env `AMQP_URI` or param `--amqpurl`)
- *CI Message Variable* This is the name of the environment variable containing the CI message send by request. It is configured on the jenkins job in the  CI subscriber. Defaults to `CI_MESSAGE`. (param `--cimessageenv`)
- *Recieve Queue Name(optional)*: The name of the queue in which replies are recieved. Defaults to `rcv_jenkinsproject_kind_target`. (env `RCV_QUEUE_NAME` or param `recievequeue`)
- *Jenkins Job/Project*: The name of jenkins project/job. (env `JOB_NAME` or param `--jenkinsproject`).
- *Jenkins URL*: The URL of the jenkins server (this should be already exposed and `JENKINS_URL` env in jenkins build or param `--jenkinsurl`)
*Jenkins Build Number*: The number of the jenkins build (this should already be exposed as `BUILD_NUMBER` in jenkins build or param `--jenkinsbuild`).
*Jenkins Robot User Name*: The name of the robot account to log into jenkins with. The user MUST be able to cancel builds for the given project. Looks for `JENKINS_ROBOT_USER` if set or param `--jenkinsuser`
- *Jenkins Robot User Password*: The password of above user. Looks for `JENKINS_ROBOT_PASSWORD` env, or can be passed as argument to cli as `--jenkinspassword`
- *Repo URL*: The cloneable repo url. (env `REPO_URL` or param `repourl`)
- *Kind*: The kind of request, can be `PR|BRANCH|TAG`. (env `KIND` or param `--kind`)
- *Target*: The target repersent what pr/branch/tag needs to be checked out. (env `TARGET` or param `--target`)
- *Setup Script(optional)*: Script that runs before the test script, to do any setup needed. (env `SETUP_SCRIPT` or param `--setupscript`)
- *Run Script*: The test script to run. (env `RUN_SCRIPT` or param `--runscript`)
- *SSH Node file(optional)*: If set to true, must have a test node db (see multi OS testing below). Can be passed to cli. (Param `--sshnodesfile file`)
- *Stand Alone(optional)*: defaults to false. In this more the worker assumes there is no request, hence no queues communication. Everything else remains same. (param `--standalone=true`). This also means you will need to provide the `CI_MESSAGE` yourself. See below for example.

## SSHNodeFile

It is possible to run your tests by sshing to other nodes that are reachable from your jenkins slave. To do so, you need to provide information in a json file, whose path, you will specify as `ci-firewall work --sshnodefile /path/to/test-nodes.json`

The format of th file is as below

```json
{
    "nodes": [
        {
            "name": "common name of node. example -Fedora 31-",
            "user": "username to ssh into the node with",
            "address": "The address of the node, like an ip or domain name without port",
            "port": "port of ssh server, optional-defaults to 22",
            "baseos": "linux|windows|mac",
            "arch": "arch of the system eg amd64",
            "password": "not recommended but you can provide password of target node",
            "privatekey": "Optional again but either this or password MUST be given."
        }
    ]
}
```

**WARNING**:  `privatekey` is the ssh private key itself. Not to be mistaken with path of the private key. Safest bet is to use a program to read content and paste it here

## Optional Standalone worker mode

This allows you to use worker in standalone mode. Note however, you will need to provide `CI_MESSAGE` yourself. See below for example.

```bash
export CI_MESSAGE='{"repourl": "repourl", "kind": "PR", "target": "target", "setupscript": "setupscript", "runscript": "runscript", "rcvident": "rcvident", "runscripturl": "http://url"}'
```
