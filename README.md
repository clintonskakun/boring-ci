# Boring CI

Minimalist CI builder for native Git.

## Features
 - Language agnostic (If it can run in bash, it's fine)
 - No pesky web UI (Just a minimal TUI and ssh login)
 - Status and log viewing
 - Secure (Builds run inside a sealed-off environment)
 - Pipelines (e.g. dev-commit, prod-deploy)
 - Async and depenant steps
 - Full contol (you decide where and how your VM runs)
 - Build steps live in the same space (no gymastics to persist across steps)

## What you need to install/configure

 - Firewall, VM, Docker, SSH
 - Githook

## Build config in your repo

.boring-ci
 - Dockerfile // Your base common environment
 - pipelines/
    dev/
    - Dockerfile // Your env for this specific pipeline FROM {BASE_IMAGE}
    - container.sh // Your container run command
    - step-1.sh // Name them whaver you want, as long as they have the .sh ext
    - step-2.sh
    - step-3.sh
    - step-4.sh
    - conf.json // Where you define the order of the build steps

container.sh
# Note: in order for this to work properly use the ${VARIABLE} fields as shown here:
docker run ${IMAGE} -v -etc ${CONTAINER} -e ... -- ${COMMAND}

step-*.sh
#!/bin/bash
# pwd is /home/builder/ as user 1001 and group 1001
# Get your stuff that's mounted via the docker volume
# Do your npm i, docker build, etc.
# exit non zero for error and zero for success

my-pipeline.json
steps:[
        [
            ["step-1.sh", "step-2.sh"], // Step 2 here waits for step 1
            ["step-3.sh"] // Step 3 runs async with both steps above
        ],
        "step-4.sh" // Step 4 runs after array of steps above have all complete
    ]

You can create as many pipelines as you want.

## Config on your server

Install sqlite3, docker-ce and go.

Create a "boring-ci" user and group and home directory.

Add boring-cli to docker group

In /home/boring-ci/.config/repos.json

{
    "myrepo": "https://mygithost.com/repo.git"
}

Add your ssh key for your repo so that boring-ci can clone it.

Install and compile:
get clone ...
go build .

Move boring-cli and boring-d to /usr/local/bin

Run the daemon(in systemd, openrc, pm2, whatever)
$ boring-d

## Set up your githook to do this (Make sure it has ssh access to the builder)

ssh boring-ci@myremotehost boring-cli trigger --repo myrepo --pipeline dev


## To see your builds do

ssh boring-cli@myremotehost boring-cli monitor

You can also put this into a bash file to avoid extra typing.

# | commit ref | pipeline    | time   | status
1 | uie9383kd  | my-pipeline | 1M 34S | Running

In that TUI you can navigate around and click into builds and see the logs

Build #1 (commit uie9383kd) my-pipeline

[Rerun] [Cancel]

Steps                 Logs
--------------------------------------------------------------------------
step-1.sh Finished  | blah blah blah
step-2.sh Running   | bla blah blah
step-3.sh Pending   | exited with code 0 in 1M 1S
step-4.sh Pending   |
