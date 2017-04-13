package metrics_test

import (
	"fmt"
	"github.com/EverythingMe/disposable-redis"
	metricsLib "github.com/FidelityInternational/diego-capacity-monitor/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/redis.v5"
	"os"
	"os/exec"
	"time"
)

var (
	timeNow       = time.Now().UnixNano()
	redisServer   *disposable_redis.Server
	redisPort     uint16
	err           error
	metric1String = `{"memory": 4000, "timestamp": 200}`
	metric2String = `{"memory": 3000, "timestamp": 300}`
	metric3String = fmt.Sprintf(`{"memory": 3000, "timestamp": %v}`, timeNow)
)

func createPopulatedMetricsObj() metricsLib.Metrics {
	messageMetrics := make(map[string]metricsLib.MessageMetric)
	messageMetrics["1"] = metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}
	messageMetrics["2"] = metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}
	messageMetrics["3"] = metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}
	staleDuration := (5 * time.Minute)
	return metricsLib.Metrics{MessageMetrics: messageMetrics, StaleDuration: staleDuration}
}

var _ = Describe("#CreateMetrics", func() {
	Context("When redis exists", func() {
		var vcapServicesJSON string
		JustBeforeEach(func() {
			os.Setenv("VCAP_SERVICES", vcapServicesJSON)
			os.Setenv("VCAP_APPLICATION", "{}")
		})

		AfterEach(func() {
			os.Unsetenv("VCAP_SERVICES")
			os.Unsetenv("VCAP_APPLICATION")
		})

		Context("and the connection is successful", func() {
			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": %v
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal", "redis"]
    }
  ]
}`, redisServer.Port())
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			It("returns a metrics control object with a redis client", func() {
				metrics := metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).ShouldNot(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})
		})

		Context("and the connection is missing a port", func() {
			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				Ω(err).Should(BeNil())
				vcapServicesJSON = `{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": 0
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal", "redis"]
    }
  ]
}`
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			It("returns a metrics control object with no valid redis client", func() {
				metrics := metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).Should(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})
		})

		Context("and the connection is missing a redis tag", func() {
			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": %v
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal"]
    }
  ]
}`, redisServer.Port())

			})

			AfterEach(func() {
				redisServer.Stop()
			})

			It("returns a metrics control object with no valid redis client", func() {
				metrics := metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).Should(BeNil())
				Ω(metrics.MessageMetrics).ShouldNot(BeNil())
			})
		})
	})

	Context("When redis does not exist", func() {
		It("creates a Metics control object", func() {
			metrics := metricsLib.CreateMetrics()
			Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
			Ω(metrics.RedisClient).Should(BeNil())
			Ω(metrics.MessageMetrics).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("Metrics", func() {
	var metrics metricsLib.Metrics

	Describe("#GetAll", func() {
		Context("When redis is used", func() {
			var vcapServicesJSON string
			JustBeforeEach(func() {
				os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				os.Setenv("VCAP_APPLICATION", "{}")
				metrics = metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).ShouldNot(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})

			AfterEach(func() {
				os.Unsetenv("VCAP_SERVICES")
				os.Unsetenv("VCAP_APPLICATION")
			})

			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				redisPort = redisServer.Port()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": %v
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal", "redis"]
    }
  ]
}`, redisPort)
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			Context("when there are no metrics", func() {
				It("returns an empty metrics object", func() {
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(0))
				})
			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					output, err := exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "1", metric1String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "2", metric2String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "3", metric3String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
				})

				It("returns a populated metrics object", func() {
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(3))
					Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})

		Context("when redis is not used", func() {
			Context("when there are no metrics", func() {
				BeforeEach(func() {
					metrics = metricsLib.CreateMetrics()
				})

				It("returns an empty metrics object", func() {
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(0))
				})
			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					metrics = createPopulatedMetricsObj()
				})

				It("returns a populated metrics object", func() {
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(3))
					Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})
	})

	Describe("#Set", func() {
		Context("When redis is used", func() {
			var vcapServicesJSON string
			JustBeforeEach(func() {
				os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				os.Setenv("VCAP_APPLICATION", "{}")
				metrics = metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).ShouldNot(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})

			AfterEach(func() {
				os.Unsetenv("VCAP_SERVICES")
				os.Unsetenv("VCAP_APPLICATION")
			})

			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				redisPort = redisServer.Port()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
	  "p-redis": [
	    {
	      "credentials": {
	        "host": "127.0.0.1",
	        "password": "",
	        "port": %v
	      },
	      "label": "p-redis",
	      "name": "diego-capacity-monitor-redis",
	      "plan": "shared-vm",
	      "provider": null,
	      "syslog_drain_url": null,
	      "tags": ["pivotal", "redis"]
	    }
	  ]
	}`, redisPort)
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			Context("when there are no metrics", func() {
				BeforeEach(func() {
					metrics = metricsLib.CreateMetrics()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(HaveLen(0))
				})

				It("adds multiple metrics", func() {
					metrics.Set("1", metricsLib.MessageMetric{Memory: 4000, Timestamp: 200})
					metrics.Set("2", metricsLib.MessageMetric{Memory: 3000, Timestamp: 300})
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(2))
					Ω(newMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
				})

			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					output, err := exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "1", metric1String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "2", metric2String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "3", metric3String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
				})

				It("overrides existing metrics", func() {
					metrics.Set("1", metricsLib.MessageMetric{Memory: 5000, Timestamp: 100})
					metrics.Set("2", metricsLib.MessageMetric{Memory: 6000, Timestamp: 200})
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(3))
					Ω(newMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 5000, Timestamp: 100}))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 6000, Timestamp: 200}))
					Ω(newMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})

		Context("When redis is not used", func() {
			Context("when there are no metrics", func() {
				BeforeEach(func() {
					metrics = metricsLib.CreateMetrics()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(HaveLen(0))
				})

				It("adds a metric", func() {
					metrics.Set("1", metricsLib.MessageMetric{Memory: 4000, Timestamp: 200})
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(1))
					Ω(newMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
				})

				It("adds multiple metrics", func() {
					metrics.Set("1", metricsLib.MessageMetric{Memory: 4000, Timestamp: 200})
					metrics.Set("2", metricsLib.MessageMetric{Memory: 3000, Timestamp: 300})
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(2))
					Ω(newMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
				})

			})
			Context("when there are metrics", func() {
				BeforeEach(func() {
					metrics = createPopulatedMetricsObj()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(HaveLen(3))
					Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})

				It("overrides existing metrics", func() {
					metrics.Set("1", metricsLib.MessageMetric{Memory: 5000, Timestamp: 100})
					metrics.Set("2", metricsLib.MessageMetric{Memory: 6000, Timestamp: 200})
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(3))
					Ω(newMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 5000, Timestamp: 100}))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 6000, Timestamp: 200}))
					Ω(newMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})
	})

	Describe("#Delete", func() {
		Context("When redis is used", func() {
			var vcapServicesJSON string
			JustBeforeEach(func() {
				os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				os.Setenv("VCAP_APPLICATION", "{}")
				metrics = metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).ShouldNot(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})

			AfterEach(func() {
				os.Unsetenv("VCAP_SERVICES")
				os.Unsetenv("VCAP_APPLICATION")
			})

			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				redisPort = redisServer.Port()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": %v
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal", "redis"]
    }
  ]
}`, redisPort)
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					output, err := exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "1", metric1String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "2", metric2String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "3", metric3String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
				})

				It("you can delete a single metrics object", func() {
					metrics.Delete("1")
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(2))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(newMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})

		Context("when redis is not used", func() {
			Context("when there are no metrics", func() {
				BeforeEach(func() {
					metrics = metricsLib.CreateMetrics()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(HaveLen(0))
				})

				It("returns an empty metrics object with no errors", func() {
					metrics.Delete("1")
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(0))
				})
			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					metrics = createPopulatedMetricsObj()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(3))
					Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})

				It("you can delete a single metrics object", func() {
					metrics.Delete("1")
					newMetrics := metrics.GetAll()
					Ω(newMetrics).Should(HaveLen(2))
					Ω(newMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(newMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})
			})
		})
	})

	Describe("#IsMetricStale", func() {
		Context("When redis is used", func() {
			var vcapServicesJSON string
			JustBeforeEach(func() {
				os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				os.Setenv("VCAP_APPLICATION", "{}")
				metrics = metricsLib.CreateMetrics()
				Ω(metrics).Should(BeAssignableToTypeOf(metricsLib.Metrics{}))
				Ω(metrics.RedisClient).ShouldNot(Equal(&redis.Client{}))
				Ω(metrics.MessageMetrics).Should(BeNil())
			})

			AfterEach(func() {
				os.Unsetenv("VCAP_SERVICES")
				os.Unsetenv("VCAP_APPLICATION")
			})

			BeforeEach(func() {
				redisServer, err = disposable_redis.NewServerRandomPort()
				redisPort = redisServer.Port()
				Ω(err).Should(BeNil())
				vcapServicesJSON = fmt.Sprintf(`{
  "p-redis": [
    {
      "credentials": {
        "host": "127.0.0.1",
        "password": "",
        "port": %v
      },
      "label": "p-redis",
      "name": "diego-capacity-monitor-redis",
      "plan": "shared-vm",
      "provider": null,
      "syslog_drain_url": null,
      "tags": ["pivotal", "redis"]
    }
  ]
}`, redisPort)
			})

			AfterEach(func() {
				redisServer.Stop()
			})

			Context("when there are metrics", func() {
				BeforeEach(func() {
					output, err := exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "1", metric1String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "2", metric2String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
					output, err = exec.Command("redis-cli", "-p", fmt.Sprintf("%v", redisPort), "set", "3", metric3String).Output()
					Ω(err).To(BeNil())
					Ω(string(output)).Should(Equal("OK\n"))
				})
				It("correctly returns the stale state of the metric", func() {
					Ω(metrics.IsMetricStale("1")).Should(BeTrue())
					Ω(metrics.IsMetricStale("2")).Should(BeTrue())
					Ω(metrics.IsMetricStale("3")).Should(BeFalse())
				})
			})
		})

		Context("when redis is not there", func() {
			Context("when there are metrics", func() {
				BeforeEach(func() {
					metrics = createPopulatedMetricsObj()
					allMetrics := metrics.GetAll()
					Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
					Ω(allMetrics).Should(HaveLen(3))
					Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
					Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
					Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
				})

				It("correctly returns the stale state of the metric", func() {
					Ω(metrics.IsMetricStale("1")).Should(BeTrue())
					Ω(metrics.IsMetricStale("2")).Should(BeTrue())
					Ω(metrics.IsMetricStale("3")).Should(BeFalse())
				})
			})
		})
	})

	Describe("#ClearStaleMetrics", func() {
		Context("when there are metrics", func() {
			BeforeEach(func() {
				metrics = createPopulatedMetricsObj()
				allMetrics := metrics.GetAll()
				Ω(allMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
				Ω(allMetrics).Should(HaveLen(3))
				Ω(allMetrics["1"]).Should(Equal(metricsLib.MessageMetric{Memory: 4000, Timestamp: 200}))
				Ω(allMetrics["2"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: 300}))
				Ω(allMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
			})

			It("deletes the metrics which are stale", func() {
				metrics.ClearStaleMetrics()
				newMetrics := metrics.GetAll()
				Ω(newMetrics).Should(BeAssignableToTypeOf(map[string]metricsLib.MessageMetric{}))
				Ω(newMetrics).Should(HaveLen(1))
				Ω(newMetrics["3"]).Should(Equal(metricsLib.MessageMetric{Memory: 3000, Timestamp: timeNow}))
			})
		})
	})
})
