# This is your base builder that should install all of the things you'll need across pipelines
# Each pipeline can also have it's own builder which will inherit from this one
# You can configure what you'll need throughout the entire build here
# This file is diffed and the builder image updated or ignored if not changed

# This image is recommended for running docker builds, runs, push, etc.
FROM nestybox/ubuntu-noble-systemd-docker:latest

# Best to always update
RUN sudo apt update && sudo apt upgrade

# Install all of the tools you'll need in steps
RUN sudo apt install gcc python nodejs etc etc

# It's best practice to run as a non root user, this prevents supply chain attacks from getting root access to your container
USER 1001:1001

# This will be where your git repo will be checked out
WORKDIR /workspace

# NOTE:
# 1. Exposing ports here is comletely useless.
# 2. No need to add a command or entrypoint as these will be ignored by the runner
