package webServer

import (
	"encoding/json"
	"fmt"
	"github.com/FidelityInternational/diego-capacity-monitor/metrics"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MessageMetric - A struct of the firhose metrics we care about
type MessageMetric struct {
	Memory    float64
	Timestamp int64
}

// Controller struct
type Controller struct {
	Metrics    metrics.Metrics
	CellMemory *float64
	Watermark  *string
	StartTime  time.Time
}

type cellReport struct {
	Index     string  `json:"index"`
	Memory    float64 `json:"memory"`
	LowMemory bool    `json:"low_memory"`
}

type report struct {
	Healthy                bool         `json:"healthy"`
	Message                string       `json:"message"`
	CellReports            []cellReport `json:"details,omitempty"`
	CellCount              int          `json:"cellCount"`
	CellMemory             float64      `json:"cellMemory"`
	Watermark              int          `json:"watermark"`
	RequestedWatermark     string       `json:"requested_watermark"`
	TotalFreeMemory        float64      `json:"totalFreeMemory"`
	WatermarkMemoryPercent float64      `json:"WatermarkMemoryPercent"`
}

// CreateController - returns a populated controller object
func CreateController(metrics metrics.Metrics, cellMemory *float64, watermark *string, startTime time.Time) *Controller {
	return &Controller{
		Metrics:    metrics,
		CellMemory: cellMemory,
		Watermark:  watermark,
		StartTime:  startTime,
	}
}

// WatermarkMemoryPercent2dp calculates watermark memory percent to 2 decimal places
func WatermarkMemoryPercent2dp(watermark int, cellCount int, cellMemory float64, totalFreeMemory float64) float64 {
	if cellCount > 0 {
		watermarkSize := (float64(watermark) * cellMemory)
		memoryExcludingWatermark := (float64(cellCount) * cellMemory) - watermarkSize
		totalFreeExcludingWatermark := totalFreeMemory - watermarkSize
		precentageFreeDuringMigration := (totalFreeExcludingWatermark / memoryExcludingWatermark) * 100
		// truncate to 2dp to golang way
		return float64(int(precentageFreeDuringMigration*100)) / 100
	}
	return 0
}

// CalculateWatermarkCellCount - Calculates the watermark cell count from an count or percent.
func (c *Controller) CalculateWatermarkCellCount(cellCount int) (int, error) {
	var watermarkCellCount int
	if strings.Contains(*c.Watermark, "%") {

		watermarkPercent := strings.Split(*c.Watermark, "%")[0]

		watermark, err := strconv.Atoi(watermarkPercent)
		if err != nil {
			return 0, err
		}
		watermarkCellCount = int((float64(cellCount) * (float64(watermark) / 100)) + 1)
	} else {
		watermark, err := strconv.Atoi(*c.Watermark)
		if err != nil {
			return 0, err
		}
		watermarkCellCount = watermark
	}

	return watermarkCellCount, nil
}

// Index - The only current endpoint, returns a json object of health and diego memory stats
func (c *Controller) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var keys []string
	messageMetrics := c.Metrics.GetAll()
	for k := range messageMetrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var (
		memLowCount     int
		cellCount       int
		totalFreeMemory float64
		report          report
		cellReports     []cellReport
		statusCode      int
	)

	for _, index := range keys {
		if !c.Metrics.IsMetricStale(index) {
			var memLow = false
			cellCount++
			totalFreeMemory += messageMetrics[index].Memory

			if messageMetrics[index].Memory < 2048 {
				memLowCount++
				memLow = true
			}

			cellReport := cellReport{Index: index, Memory: messageMetrics[index].Memory, LowMemory: memLow}
			cellReports = append(cellReports, cellReport)
		}
	}

	report.CellMemory = *c.CellMemory
	report.CellCount = cellCount
	report.TotalFreeMemory = totalFreeMemory
	report.RequestedWatermark = *c.Watermark

	watermarkCellCount, err := c.CalculateWatermarkCellCount(cellCount)
	if err != nil {
		report.Message = fmt.Sprintf("Error occurred while calculating cell count: %v", err.Error())
		report.write(w, http.StatusInternalServerError)
		return
	}

	report.Watermark = watermarkCellCount

	if c.Metrics.RedisNotUsed() && time.Now().Before(c.StartTime.Add(1*time.Minute)) {
		report.Message = "I'm still initialising, please be patient!"
		statusCode = http.StatusExpectationFailed
	} else if cellCount == 0 {
		report.Message = "I'm sorry Dave I can't show you any data"
		statusCode = http.StatusGone
		// Panic if we dont have more cells than the watermark
	} else if cellCount <= watermarkCellCount {
		report.Message = "The number of cells needs to exceed the watermark amount!"
		statusCode = http.StatusExpectationFailed
		// Panic if half or more of the cells are low on memory
	} else {
		WatermarkMemoryPercent := WatermarkMemoryPercent2dp(watermarkCellCount, cellCount, *c.CellMemory, totalFreeMemory)
		report.WatermarkMemoryPercent = WatermarkMemoryPercent

		// Panic if we do not have enough headroom after watermark cells are discounted
		if WatermarkMemoryPercent <= 0 {
			report.Message = "FATAL - There is not enough space to do an upgrade, add cells or reduce watermark!"
			statusCode = http.StatusExpectationFailed
		} else if WatermarkMemoryPercent < 20 {
			report.Message = "The percentage of free memory will be too low during a migration!"
			statusCode = http.StatusExpectationFailed
		} else {
			report.Message = "Everything is awesome!"
			statusCode = http.StatusOK
		}
	}
	report.CellReports = cellReports
	report.write(w, statusCode)
}

func (r *report) write(w http.ResponseWriter, statusCode int) {
	if statusCode == 200 {
		r.Healthy = true
	}
	w.WriteHeader(statusCode)
	bytes, _ := json.Marshal(r)
	fmt.Fprintf(w, "%v", string(bytes))
}
