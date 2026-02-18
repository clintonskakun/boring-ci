# THIS FILE IS OPTIONAL
# If your base image already has everything you need you can delete this file for this pipeline

# This is your builder that will run your steps for this pipeline
# You can configure what you'll need throughout the entire build here
# This file is diffed and the builder image updated or ignored if not changed

# This is your base image configured at the top level of the .bording-ci dir
FROM ${BASE_IMAGE}

# Install all of the tools you'll need for this pipeline
RUN sudo apt install gcc python nodejs etc etc

# It's best practice to run as a non root user, this prevents supply chain attacks from getting root access to your container
USER 1001:1001


# NOTE:
# 1. Exposing ports here is comletely useless.
# 2. No need to add a command or entrypoint as these will be ignored by the runner
# 3. No need to create a user or WORKDIR if you've already done this in your base image
