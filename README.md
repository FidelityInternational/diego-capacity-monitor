# diego-capacity-monitor

[![codecov.io](https://codecov.io/github/FidelityInternational/diego-capacity-monitor/coverage.svg?branch=master)](https://codecov.io/github/FidelityInternational/diego-capacity-monitor?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/FidelityInternational/diego-capacity-monitor)](https://goreportcard.com/report/github.com/FidelityInternational/diego-capacity-monitor)
[![Build Status](https://travis-ci.org/FidelityInternational/diego-capacity-monitor.svg?branch=master)](https://travis-ci.org/FidelityInternational/diego-capacity-monitor)

NOTE: This is now deprecated and not being updated for security defects. It may be deleted at any time

`diego-capacity-monitor` is a [Cloud Foundry](https://www.cloudfoundry.org) deployable web application that subscribes to the [CF Firehose](https://docs.cloudfoundry.org/loggregator/architecture.html#firehose) to gather memory metrics about Diego cells. It then reports health states based a number of key metrics described below.

![Diego Monitor](diego-monitor.jpg "Diego Monitor")

### Operation

If there are no errors this application will return a json response similar to:

```
{
  healthy: true,
  message:"Everything is awesome!",
  details:[
    {
      index: 1,
      memory: 7000,
      low_memory: false
    },
    {
      index: 2,
      memory: 7000,
      low_memory: false
    }
  ]
  cellCount: 2,
  cellMemory: 10000,
  watermark: 1,
  requested_watermark: "1",
  totalFreeMemory: 14000,
  WatermarkMemoryPercent: 40
}
```

The following error messages and status can also be received:

- Its under a minute since the system was started
    - report.Message = "I'm still initialising, please be patient!"
    - report.Healthy = false
    - status = http.StatusExpectationFailed
- Invalid Watermark value supplied
    - reports.Message = "Error occurred while calculating cell count"
    - report.Healthy = false
    - status = http.StatusInternalServerError
- No metrics were found
    - report.Message = "I'm sorry Dave I can't show you any data"
    - report.Healthy = false
    - status = http.StatusGone
- The cellCount is not more than the watermark count
    - report.Message = "The number of cells needs to exceed the watermark amount!"
    - report.Healthy = false
    - status = http.StatusExpectationFailed
- There is less memory free than the watermark amount
    - report.Message = "FATAL - There is not enough space to do an upgrade, add cells or reduce watermark!"
    - report.Healthy = false
    - status = http.StatusExpectationFailed
- During an upgrade there would be less than 20% memory free
    - report.Message = "The percentage of free memory will be too low during a migration!"
    - report.Healthy = false
    - status = http.StatusExpectationFailed

### Deployment

#### Watermark value

The watermark value is the number of Diego cells that will be excluded from the remaining capacity calculation, the intention is for this value to match the number of cells you would upgrade in parallel when performing a `bosh deploy`. Based on this theory the `WatermarkMemoryPercent` will show a percentage of spare load during an upgrade event, to ensure app migrations can happen in a timely manner between draining cells.

This value can be supplied either as the number of cells to upgrade in parallel, or as a percentage. It has a default value of `1`.

Example:

If we had 50 Diego Cells

`WATERMARK: 10%` - Watermark count = `5`
`WATERMARK: 10` - Watermark count = `10`

#### cf cli version

With the inclusion of stack support in the cf push you will need to be using v6.39.1 or newer of the cf cli.

#### Manual deployment

```
cf target -o <my_org> -s <my_space>
cf push --no-start
cf set-env diego-capacity-monitor CF_API_ENDPOINT <https://api.system.domain.cf>
cf set-env diego-capacity-monitor CF_USERNAME <CF_USERNAME_FOR_FIREHOSE_CONNECTION>
cf set-env diego-capacity-monitor CF_PASSWORD <CF_PASSWORD_FOR_FIREHOSE_CONNECTION>
cf set-env diego-capacity-monitor WATERMARK <optional, value will default to 1>
cf start diego-capacity-monitor
```

#### Automated zero-downtime deployment

```
CF_SYS_DOMAIN=system.example.cf.com \
CF_DEPLOY_USERNAME=cf_admin \
CF_DEPLOY_PASSWORD=123456789abcdef \
ORG_NAME=my_org \
SPACE_NAME=my_space \
CF_API_ENDPOINT=https://api.system.domain.cf \
CF_USERNAME=cf_firehose_username \
CF_PASSWORD=cf_firehose_password \
APP_NAME=my_diego_capacity_monitoring_app \
STACK=cflinuxfs2 \
./deploy.sh
```

### Testing

#### Prereqs

```
brew install redis
go get github.com/EverythingMe/disposable-redis
go get github.com/onsi/gingko/ginkgo
```

### To test and check coverage
```
ginkgo -r -cover
```

#### Smoke Tests

```
APP_URL=<diego-capacity-monitor.apps.example.com> \
./smoke_test.sh
```
