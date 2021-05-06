
# Development Workflow

## Prerequisites

* You have Go 1.14 installed on your local development machine.
* You have Docker installed on your local development machine. Docker is required for building velero-plugin container images and to push them into a Kubernetes cluster for testing.
* You have `kubectl` installed. For running integration tests, you will require an existing single node cluster with [openebs](https://blog.openebs.io/how-to-install-openebs-with-kubernetes-using-minikube-2ed488dff1c2) and [velero](https://velero.io/docs/master/basic-install/) installed. Don't worry if you don't have access to the Kubernetes cluster, raising a PR with the velero-plugin repository will run integration tests for your changes against a Minikube cluster.

## Initial Setup

### Fork in the cloud

1. Visit <https://github.com/openebs/velero-plugin>
2. Click the `Fork` button (top right) to establish a cloud-based fork.

### Clone fork to the local machine

Place openebs/velero-plugin's code on your `GOPATH` using the following cloning procedure.
Create your clone:

```sh

mkdir -p $GOPATH/src/github.com/openebs
cd $GOPATH/src/github.com/openebs

# Note: Here user= your github profile name
git clone https://github.com/$user/velero-plugin.git

# Configure remote upstream
cd $GOPATH/src/github.com/openebs/velero-plugin
git remote add upstream https://github.com/openebs/velero-plugin.git

# Never push to upstream master
git remote set-url --push upstream no_push

# Confirm that your remotes make sense:
git remote -v
```

> **Note:** If your `GOPATH` has more than one (`:` separated) paths in it, then you should use *one of your go path* instead of `$GOPATH` in the commands mentioned here. This statement holds throughout this document.

### Building and Testing your changes

* To build the velero-plugin binary

```sh

make
```

* To build the docker image

```sh

make container REPO=<YOUR_REPO>
```

* Test your changes
Integration tests are written in ginkgo and run against a minikube cluster. The Minikube cluster should be running to execute the tests. To install the Minikube follow the doc [here](https://kubernetes.io/docs/tasks/tools/install-minikube/).
To run the integration tests on the minikube cluster.

```sh
make tet
```

## Git Development Workflow

### Always sync your local repository

Open a terminal on your local machine. Change directory to the velero-plugin fork root.

```sh
cd $GOPATH/src/github.com/openebs/velero-plugin
```

 Check out the master branch.

 ```sh
 $ git checkout master
 Switched to branch 'master'
 Your branch is up-to-date with 'origin/master'.
 ```

 Recall that origin/master is a branch on your remote GitHub repository.
 Make sure you have the upstream remote openebs/velero-plugin by listing them.

 ```sh
 $ git remote -v
 origin https://github.com/$user/velero-plugin.git (fetch)
 origin https://github.com/$user/velero-plugin.git (push)
 upstream https://github.com/openebs/velero-plugin.git (fetch)
 upstream https://github.com/openebs/velero-plugin.git (no_push)
 ```

 If the upstream is missing, add it by using the below command.

 ```sh
 git remote add upstream https://github.com/openebs/velero-plugin.git
 ```

 Fetch all the changes from the upstream master branch.

 ```sh
 $ git fetch upstream master
 remote: Counting objects: 141, done.
 remote: Compressing objects: 100% (29/29), done.
 remote: Total 141 (delta 52), reused 46 (delta 46), pack-reused 66
 Receiving objects: 100% (141/141), 112.43 KiB | 0 bytes/s, done.
 Resolving deltas: 100% (79/79), done.
 From github.com:openebs/velero-plugin
   * branch            master     -> FETCH_HEAD
 ```

 Rebase your local master with the upstream/master.

 ```sh
 $ git rebase upstream/master
 First, rewinding head to replay your work on top of it...
 Fast-forwarded master to upstream/master.
 ```

 This command applies all the commits from the upstream master to your local master.

 Check the status of your local branch.

 ```sh
 $ git status
 On branch master
 Your branch is ahead of 'origin/master' by 12 commits.
 (use "git push" to publish your local commits)
 nothing to commit, working directory clean
 ```

 Your local repository now has all the changes from the upstream remote. You need to push the changes to your remote fork which is origin master.

 Push the rebased master to origin master.

 ```sh
 $ git push origin master
 Username for 'https://github.com': $user
 Password for 'https://$user@github.com':
 Counting objects: 223, done.
 Compressing objects: 100% (38/38), done.
 Writing objects: 100% (69/69), 8.76 KiB | 0 bytes/s, done.
 Total 69 (delta 53), reused 47 (delta 31)
 To https://github.com/$user/velero-plugin.git
 8e107a9..5035fa1  master -> master
 ```

### Contributing to a feature or bugfix

Always start with creating a new branch from master to work on a new feature or bugfix. Your branch name should have the format XX-descriptive where XX is the issue number you are working on followed by some descriptive text. For example:

 ```sh
 $ git checkout master
 # Make sure the master is rebased with the latest changes as described in the previous step.
 $ git checkout -b 1234-fix-developer-docs
 Switched to a new branch '1234-fix-developer-docs'
 ```

Happy Hacking!

### Keep your branch in sync

[Rebasing](https://git-scm.com/docs/git-rebase) is very important to keep your branch in sync with the changes being made by others and to avoid huge merge conflicts while raising your Pull Requests. You will always have to rebase before raising the PR.

```sh
# While on your myfeature branch (see above)
git fetch upstream
git rebase upstream/master
```

While you rebase your changes, you must resolve any conflicts that might arise and build and test your changes using the above steps.

## Submission

### Create a pull request

Before you raise the Pull Requests, ensure you have reviewed the checklist in the [CONTRIBUTING GUIDE](CONTRIBUTING.md):

* Ensure that you have re-based your changes with the upstream using the steps above.
* Ensure that you have added the required unit tests for the bug fix or a new feature that you have introduced.
* Ensure your commit history is clean with proper header and descriptions.

Go to the [openebs/velero-plugin github](https://github.com/openebs/velero-plugin) and follow the Open Pull Request link to raise your PR from your development branch.
