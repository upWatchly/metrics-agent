# [1.2.0](https://github.com/upWatchly/metrics-agent/compare/v1.1.0...v1.2.0) (2026-07-14)


### Features

* implement partition filtering for Windows and non-Windows platforms ([a5c3d02](https://github.com/upWatchly/metrics-agent/commit/a5c3d0250a61ec4a47020f048a263e55d2f3eb57))

# [1.1.0](https://github.com/upWatchly/metrics-agent/compare/v1.0.0...v1.1.0) (2026-07-14)


### Features

* enhance agent reporting with error handling and disk usage management ([8002a0f](https://github.com/upWatchly/metrics-agent/commit/8002a0f8b669c7342a3de2c3c5ea932f2965c844))

# 1.0.0 (2026-07-14)


### Bug Fixes

* set default API endpoint for configuration loading ([d184e71](https://github.com/upWatchly/metrics-agent/commit/d184e71c126e4d5eb0772249098e0e0ad8075d92))
* update metrics structure and API client for improved data handling ([a3f7057](https://github.com/upWatchly/metrics-agent/commit/a3f7057384bba32e1f0a111d9a1d7207e9694669))


### Features

* add agent version tracking and improve CPU metrics reporting ([6d31874](https://github.com/upWatchly/metrics-agent/commit/6d318742b0ba3f41c893753b8ed24095b13f207c))
* add detailed logging for server configuration and report responses ([e56e815](https://github.com/upWatchly/metrics-agent/commit/e56e81503272fca8451f58202fd9bdb2a996548a))
* add Docker containers metrics collection to report ([ddc3ed7](https://github.com/upWatchly/metrics-agent/commit/ddc3ed7cf805cef68eb241bcf1e10c1480d6ec72))
* add environment variables for host directories in Dockerfile ([901a511](https://github.com/upWatchly/metrics-agent/commit/901a5115a1ad8c3e98b65ce4060e3022ac29a943))
* add environment variables for log level and HTTP keep-alive configuration in install script ([fcae5ff](https://github.com/upWatchly/metrics-agent/commit/fcae5ff4b8e55a1383522d2938d074ac092f2231))
* add installation script for Upwatchly Metrics Agent ([80f7131](https://github.com/upWatchly/metrics-agent/commit/80f71311fb4636a6f7436ad2c9eb9b1112612950))
* add pprof server for performance profiling ([83cdf98](https://github.com/upWatchly/metrics-agent/commit/83cdf98076e3fd03c45282240def7f54b7492083))
* add support for disabling HTTP keep-alive and improve logging configuration ([913978f](https://github.com/upWatchly/metrics-agent/commit/913978f4c5c9c904f35162251458aa31fa6a0214))
* add Windows service support and installer scripts ([24a4cfc](https://github.com/upWatchly/metrics-agent/commit/24a4cfcd06afe8d5bbd49c41aec92c6ca974d7a7))
* conditionally start pprof server based on environment variable ([266349c](https://github.com/upWatchly/metrics-agent/commit/266349c9af47dd57b8c48ab5c729d88c8fd5abc8))
* detect public IPs and enhance installation script for updates ([545a05d](https://github.com/upWatchly/metrics-agent/commit/545a05dff154cb3cee01d58c409453861a72241d))
* enhance installation script and release workflow for improved release handling ([cb4a476](https://github.com/upWatchly/metrics-agent/commit/cb4a47646088fd6dadad99a9453cfd4e99e227cd))
* enhance metrics collection and reporting with live mode support ([fdb90c6](https://github.com/upWatchly/metrics-agent/commit/fdb90c6c85c047fbf94befa3097b96155b7ebb7e))
* enhance network metrics reporting by capping anomalous spikes ([65df1bd](https://github.com/upWatchly/metrics-agent/commit/65df1bdb1e85d271181c94e841d565cd3eee89cc))
* implement Docker API integration for container stats collection ([f1b73cc](https://github.com/upWatchly/metrics-agent/commit/f1b73cc1ce7971f38cc168c6f0d22711bcee13ea))
* implement reusable Docker HTTP client in collector ([2a747fb](https://github.com/upWatchly/metrics-agent/commit/2a747fbd4ea2e0d736552b811748dcf0fc09553a))
* metrics agent for server monitoring ([f40586a](https://github.com/upWatchly/metrics-agent/commit/f40586af2b7ae767dba76ec1f5bc1972818113b0))
* optimize CPU wait time and configure Go memory limits in Dockerfile ([2ee966d](https://github.com/upWatchly/metrics-agent/commit/2ee966d43dcb77fe3480660ed009d9161c1c5989))
* remove redundant logging for server configuration and report responses ([68ddd2e](https://github.com/upWatchly/metrics-agent/commit/68ddd2e599a4aa6778bba315e372ca2fea53510d))
* replace time.After with time.Ticker for periodic data collection ([192f3c1](https://github.com/upWatchly/metrics-agent/commit/192f3c1d765956537235e9c7608b96d4348b81f5))
* stop service before updating binary in installation script ([eea0c0b](https://github.com/upWatchly/metrics-agent/commit/eea0c0b00ac16c2ae503576dec110dccc5798d7a))
* update context handling for metric sending and server reporting ([c7268eb](https://github.com/upWatchly/metrics-agent/commit/c7268eb8c10214a104870932406ec2021d259694))
