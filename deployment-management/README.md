# deployment-management

This is a deployment management internal app for managing bitswan depoyments.

There are different types of deployments.

# Gitops and editor

There are two env vars set DOCKER_PASSWORD and DOCKER_USER

We have 4 repos:

- https://hub.docker.com/r/bitswan/bitswan-editor-staging
- https://hub.docker.com/r/bitswan/gitops-staging
- https://hub.docker.com/r/bitswan/bitswan-editor
- https://hub.docker.com/r/bitswan/gitops


Each staging repo has docker tags in the form of:

- bitswan/bitswan-editor-staging:latest
- bitswan/bitswan-editor-staging:sha256_ca1b419eae8e2156408daa075a13c2c7c6c7ce1cb34864bc7c3e7891d5f83b5f
- bitswan/bitswan-editor-staging:2026-23140813591-git-c9cd8f5

The user should be able to see a list of staging and non staging releases. And push the staging releases to production with a button in the UI. This should set all three tags in production to the image that we selected to release from staging. We should be able to see which images from staging are released to production.

# Automation server

Automation server deployments are exposed here https://github.com/bitswan-space/bitswan-automation-server via github releases. The deployment manager should allow us to put a deployment into production by downloading the selected release binaries and making them publicly available for download from the external app. The external app should include installation instructions on setting up the automation server.