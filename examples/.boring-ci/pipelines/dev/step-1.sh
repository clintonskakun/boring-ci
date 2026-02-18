#!/bin/bash

echo "I'm in ${REPO_NAME} ${BRANCH_NAME} ${COMMIT_SHA} or ${COMMIT_SHA_SHORT} and my pipeline is named ${PIPELINE_NAME} and I'm in step ${STEP_NAME}. I was started because ${COMMIT_AUTHOR_NAME} with email ${COMMIT_AUTHOR_EMAIL} pushed this commit with message ${COMMIT_MESSAGE}. You can see it all here in ls . which is /workspace. I can also build, pull, run and push my docker images."
