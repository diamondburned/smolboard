# smolboard

[![pipeline status](https://gitlab.com/diamondburned/smolboard/badges/renai/pipeline.svg)](https://gitlab.com/diamondburned/smolboard/-/commits/renai)
[![coverage report](https://gitlab.com/diamondburned/smolboard/badges/renai/coverage.svg)](https://gitlab.com/diamondburned/smolboard/-/commits/renai)

## Hosting

smolboard's backend can be hosted separately from the frontend.

### Backend

smolboard's backend requires a Unix socket to listen to. This is done to force
usage of a reverse proxy that uses X-Forwarded-For.

The default config at `./config.default.toml` should have sane defaults to
start. To override this config, make a new `./config.toml` file and change that.

If this is the first launch, it is also advised to run `./smolboard create-owner`,
which would create a new owner account with the username inside the config. Note
that if the username inside the config is changed, then the old user will become
a regular user, and the new user will become the owner.

To disable the frontend and host them separately, run the backend with `-nf` and
refer to the frontend section below.

#### Dependencies

- libsqlite3
- libjpeg
- FFmpeg (optional)
- FFprobe (optional)

### Frontend

smolboard's frontend can also be hosted separately from the backend. To do so,
compile `./frontend/` and run it separately.

The frontend has a default config at `./frontend/config.default.toml` that one
needs to change. Specifically, `listenAddress` should be the HTTP address that
the frontend should listen on, and `backendAddress` should be the HTTP address
that points to the backend.

#### Dependencies

- libjpeg

