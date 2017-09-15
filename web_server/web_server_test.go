package webServer_test

import (
	metricsLib "github.com/FidelityInternational/diego-capacity-monitor/metrics"
	webs "github.com/FidelityInternational/diego-capacity-monitor/web_server"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"time"
)

type cellMemory float64

func Router(controller *webs.Controller) *mux.Router {
	server := &webs.Server{Controller: controller}
	r := server.Start()
	return r
}

func init() {
	var controller *webs.Controller
	http.Handle("/", Router(controller))
}

var _ = Describe("Server", func() {
	Describe("#CreateServer", func() {
		var (
			messageMetrics map[string]metricsLib.MessageMetric
			cellMemory     float64
			watermark      string
		)

		It("returns a server object", func() {
			Ω(webs.CreateServer(metricsLib.Metrics{MessageMetrics: messageMetrics}, &cellMemory, &watermark)).Should(BeAssignableToTypeOf(&webs.Server{}))
		})
	})
})

var _ = Describe("Controller", func() {
	Describe("#CreateController", func() {
		var (
			messageMetrics map[string]metricsLib.MessageMetric
			cellMemory     float64
			watermark      string
			startTime      time.Time
		)

		It("returns a controller object", func() {
			controller := webs.CreateController(metricsLib.Metrics{MessageMetrics: messageMetrics}, &cellMemory, &watermark, startTime)
			Ω(controller).Should(BeAssignableToTypeOf(&webs.Controller{}))
		})
	})

	Describe("#Index", func() {
		var (
			cellMemory   float64
			watermark    string
			startTime    time.Time
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
			metrics      metricsLib.Metrics
			timeNow      = time.Now().UnixNano()
		)

		JustBeforeEach(func() {
			mockRecorder = httptest.NewRecorder()
			controller = webs.CreateController(metrics, &cellMemory, &watermark, startTime)
			req, _ = http.NewRequest("GET", "http://example.com/", nil)
			Router(controller).ServeHTTP(mockRecorder, req)
		})

		Context("when the watermark is invalid", func() {
			BeforeEach(func() {
				metrics = metricsLib.CreateMetrics()
				cellMemory = 10000
				watermark = "invalid"
				startTime = time.Now().Add(-1 * time.Minute)
			})

			It("reports healthy as false with a report message as an error", func() {
				Ω(mockRecorder.Code).To(Equal(500))
				Ω(mockRecorder.Body.String()).Should(MatchRegexp(`{"healthy":false,"message":"Error occurred while calculating cell count: ` +
					`strconv\..*: parsing \\"invalid\\": invalid syntax","cellCount":0,"cellMemory":10000,"watermark":0,` +
					`"requested_watermark":"invalid","totalFreeMemory":0,"WatermarkMemoryPercent":0}`))
			})
		})

		Context("when watermark is valid", func() {
			BeforeEach(func() {
				metrics = metricsLib.CreateMetrics()
				cellMemory = 10000
				watermark = "1"
				startTime = time.Now().Add(-1 * time.Minute)
			})

			AfterEach(func() {
				watermark = "1"
			})

			Context("when there are no metrics at all", func() {
				BeforeEach(func() {
					metrics.Delete("1")
				})

				It("reports healthy as false", func() {
					Ω(mockRecorder.Code).To(Equal(410))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"I'm sorry Dave I can't show you any data",` +
						`"cellCount":0,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":0,"WatermarkMemoryPercent":0}`))
				})
			})

			Context("when there are metrics", func() {
				Context("and all metrics are stale", func() {
					BeforeEach(func() {
						metrics.Set("1", metricsLib.MessageMetric{Memory: 5000, Timestamp: 200})
					})

					It("reports healthy as false", func() {
						Ω(mockRecorder.Code).To(Equal(410))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"I'm sorry Dave I can't show you any data",` +
							`"cellCount":0,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":0,"WatermarkMemoryPercent":0}`))
					})
				})

				Context("and a metric is not stale", func() {
					Context("and the system is initialiseing", func() {
						BeforeEach(func() {
							startTime = time.Now()
							metrics.Set("1", metricsLib.MessageMetric{Memory: 1000, Timestamp: timeNow})
						})

						It("reports healthy as false", func() {
							Ω(mockRecorder.Code).To(Equal(417))
							Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"I'm still initialising, please be patient!","details":[` +
								`{"index":"1","memory":1000,"low_memory":true}` +
								`],"cellCount":1,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":1000,"WatermarkMemoryPercent":0}`))
						})
					})

					Context("and memory is below the threshold", func() {
						Context("and memory is below the threshold on at at least half the cells", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 4000, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 2000, Timestamp: timeNow})
							})

							It("reports healthy as false", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"At least a third of the cells are low on memory!","details":[` +
									`{"index":"1","memory":4000,"low_memory":false},` +
									`{"index":"2","memory":2000,"low_memory":true}` +
									`],"cellCount":2,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":6000,"WatermarkMemoryPercent":0}`))
							})
						})

						Context("and memory is below the threshold on too many cells", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 4000, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 4000, Timestamp: timeNow})
								metrics.Set("3", metricsLib.MessageMetric{Memory: 4000, Timestamp: timeNow})
								metrics.Set("4", metricsLib.MessageMetric{Memory: 4000, Timestamp: timeNow})
								metrics.Set("5", metricsLib.MessageMetric{Memory: 2000, Timestamp: timeNow})
								metrics.Set("6", metricsLib.MessageMetric{Memory: 2000, Timestamp: timeNow})
							})

							It("reports healthy as false", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"At least a third of the cells are low on memory!","details":[` +
									`{"index":"1","memory":4000,"low_memory":false},` +
									`{"index":"2","memory":4000,"low_memory":false},` +
									`{"index":"3","memory":4000,"low_memory":false},` +
									`{"index":"4","memory":4000,"low_memory":false},` +
									`{"index":"5","memory":2000,"low_memory":true},` +
									`{"index":"6","memory":2000,"low_memory":true}` +
									`],"cellCount":6,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":20000,"WatermarkMemoryPercent":0}`))
							})
						})
					})

					Context("and memory is above the threshold", func() {
						BeforeEach(func() {
							metrics.Set("1", metricsLib.MessageMetric{Memory: 6321, Timestamp: timeNow})
							metrics.Set("2", metricsLib.MessageMetric{Memory: 6321, Timestamp: timeNow})
						})

						It("reports healthy as true", func() {
							Ω(mockRecorder.Code).To(Equal(200))
							Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":true,"message":"Everything is awesome!","details":[` +
								`{"index":"1","memory":6321,"low_memory":false},` +
								`{"index":"2","memory":6321,"low_memory":false}` +
								`],"cellCount":2,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":12642,"WatermarkMemoryPercent":26.42}`))
						})
					})

					Context("and there are not enough cells that specified watermark value", func() {
						Context("with no stale data", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 6000, Timestamp: timeNow})
							})

							It("reports healthy as true", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"The number of cells needs to exceed the watermark amount!","details":[` +
									`{"index":"1","memory":6000,"low_memory":false}` +
									`],"cellCount":1,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":6000,"WatermarkMemoryPercent":0}`))
							})
						})

						Context("with stale data", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 6000, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 6000, Timestamp: 200})
							})

							It("reports healthy as true", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"The number of cells needs to exceed the watermark amount!","details":[` +
									`{"index":"1","memory":6000,"low_memory":false}` +
									`],"cellCount":1,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":6000,"WatermarkMemoryPercent":0}`))
							})
						})
					})

					Context("and there are more cells than the watermark value", func() {
						Context("and there is not enough free memory to do a migration at all", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 2100, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 2100, Timestamp: timeNow})
								metrics.Set("3", metricsLib.MessageMetric{Memory: 2100, Timestamp: timeNow})
								metrics.Set("4", metricsLib.MessageMetric{Memory: 2100, Timestamp: timeNow})
							})

							It("reports healthy as false", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"FATAL - There is not enough space to do an upgrade, add cells or reduce watermark!","details":[` +
									`{"index":"1","memory":2100,"low_memory":false},` +
									`{"index":"2","memory":2100,"low_memory":false},` +
									`{"index":"3","memory":2100,"low_memory":false},` +
									`{"index":"4","memory":2100,"low_memory":false}` +
									`],"cellCount":4,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":8400,"WatermarkMemoryPercent":-5.33}`))
							})
						})

						Context("and there is not enough free memory to safely do a migration", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 3100, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 3100, Timestamp: timeNow})
								metrics.Set("3", metricsLib.MessageMetric{Memory: 3100, Timestamp: timeNow})
								metrics.Set("4", metricsLib.MessageMetric{Memory: 3100, Timestamp: timeNow})
							})

							It("reports healthy as false", func() {
								Ω(mockRecorder.Code).To(Equal(417))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":false,"message":"The percentage of free memory will be too low during a migration!","details":[` +
									`{"index":"1","memory":3100,"low_memory":false},` +
									`{"index":"2","memory":3100,"low_memory":false},` +
									`{"index":"3","memory":3100,"low_memory":false},` +
									`{"index":"4","memory":3100,"low_memory":false}` +
									`],"cellCount":4,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":12400,"WatermarkMemoryPercent":8}`))
							})
						})

						Context("and there is enough free memory", func() {
							BeforeEach(func() {
								metrics.Set("1", metricsLib.MessageMetric{Memory: 5000, Timestamp: timeNow})
								metrics.Set("2", metricsLib.MessageMetric{Memory: 5000, Timestamp: timeNow})
								metrics.Set("3", metricsLib.MessageMetric{Memory: 5000, Timestamp: timeNow})
							})

							It("reports healthy as true", func() {
								Ω(mockRecorder.Code).To(Equal(200))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"healthy":true,"message":"Everything is awesome!","details":[` +
									`{"index":"1","memory":5000,"low_memory":false},` +
									`{"index":"2","memory":5000,"low_memory":false},` +
									`{"index":"3","memory":5000,"low_memory":false}` +
									`],"cellCount":3,"cellMemory":10000,"watermark":1,"requested_watermark":"1","totalFreeMemory":15000,"WatermarkMemoryPercent":25}`))
							})
						})
					})
				})
			})
		})
	})

	Describe("#CalculateWatermarkCellCount", func() {
		var (
			controller *webs.Controller
		)

		BeforeEach(func() {
			controller = &webs.Controller{}
		})

		Context("when a watermark is supplied as an int", func() {
			Context("and the value supplied is invalid", func() {
				BeforeEach(func() {
					watermark := "invalid"
					controller.Watermark = &watermark
				})

				It("returns the specified watermark cell count value as an int", func() {
					cellCount, err := controller.CalculateWatermarkCellCount(4)
					Ω(cellCount).Should(Equal(0))
					Ω(err).ShouldNot(BeNil())
					Ω(err.Error()).Should(MatchRegexp(`strconv\..*: parsing "invalid": invalid syntax`))
				})
			})

			Context("and the value supplied is valid", func() {
				BeforeEach(func() {
					watermark := "10"
					controller.Watermark = &watermark
				})

				It("returns the specified watermark cell count value as an int", func() {
					cellCount, err := controller.CalculateWatermarkCellCount(5454)
					Ω(cellCount).Should(Equal(10))
					Ω(err).Should(BeNil())
				})
			})
		})

		Context("when a watermark is supplied as a percent", func() {
			Context("and the value supplied is invalid", func() {
				BeforeEach(func() {
					watermark := "invalid%"
					controller.Watermark = &watermark
				})

				It("returns the specified watermark cell count value as an int", func() {
					cellCount, err := controller.CalculateWatermarkCellCount(4)
					Ω(cellCount).Should(Equal(0))
					Ω(err).ShouldNot(BeNil())
					Ω(err.Error()).Should(MatchRegexp(`strconv\..*: parsing "invalid": invalid syntax`))
				})
			})

			Context("and the value supplied is valid", func() {
				BeforeEach(func() {
					watermark := "10%"
					controller.Watermark = &watermark
				})

				It("returns the specified watermark cell count value as an int", func() {
					cellCount, err := controller.CalculateWatermarkCellCount(4)
					Ω(cellCount).Should(Equal(1))
					Ω(err).Should(BeNil())
					cellCount, err = controller.CalculateWatermarkCellCount(56)
					Ω(cellCount).Should(Equal(6))
					Ω(err).Should(BeNil())
					cellCount, err = controller.CalculateWatermarkCellCount(41)
					Ω(cellCount).Should(Equal(5))
					Ω(err).Should(BeNil())
					cellCount, err = controller.CalculateWatermarkCellCount(99)
					Ω(cellCount).Should(Equal(10))
					Ω(err).Should(BeNil())
				})
			})
		})
	})
})

var _ = Describe("#WatermarkMemoryPercent2dp", func() {
	var (
		percent   float64
		cellCount int
	)

	JustBeforeEach(func() {
		percent = webs.WatermarkMemoryPercent2dp(0, cellCount, 3, 1)
	})

	Context("when cellCount is 0", func() {
		BeforeEach(func() {
			cellCount = 0
		})

		It("returns 0", func() {
			Ω(percent).Should(Equal(float64(0)))
		})
	})

	Context("when cellCount is less than 0", func() {
		BeforeEach(func() {
			cellCount = -20
		})
		It("returns 0", func() {
			Ω(percent).Should(Equal(float64(0)))
		})
	})

	Context("when cellCount is greater than 0", func() {
		BeforeEach(func() {
			cellCount = 1
		})

		It("returns the percentage", func() {
			Ω(percent).Should(Equal(33.33))
		})
	})
})
