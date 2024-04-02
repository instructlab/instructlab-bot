# Troubleshooting Help

## MacOs Dev Environment

- `make run-dev` throws following error

        ```text
        error getting credentials - err: exec: "docker-credential-osxkeychain": executable file not found in $PATH, out: ``
        ```

        If your dev machine has docker-compose installed, Podman compose will by default uses docker-compose to run the services. The error is because the docker-compose is not able to find the docker credential helper. To fix this, you need to install the docker credential helper.

        ```bash
        brew install docker-credential-helper
        ```

- `make run-dev` throws following error

        ```text
        ✘ bot Error         {"message":"unable to retrieve auth token: invalid username/password: unauthorized"}                                                                                                                                                                  1.1s
        ✘ worker-test Error {"message":"unable to retrieve auth token: invalid username/password: unauthorized"}                                                                                                                                                                  1.1s
        ✘ redis Error       context canceled                                                                                                                                                                                                                                      9.3s
        Error response from daemon: {"message":"unable to retrieve auth token: invalid username/password: unauthorized"}
        Error: executing /usr/local/bin/docker-compose up: exit status 18
        ```

        Make sure podman desktop is configured with ghcr.io registry. To check this, open the podman desktop dashboard, and go to Settings -> Registries. Select the Github Container Registry and make sure the credentials are set. You will have to use the Personal Access Token to authenticate with the Github Container Registry. Once Github Container Registry is configured, try running `make run-dev` again.
