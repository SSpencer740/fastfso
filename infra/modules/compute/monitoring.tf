resource "google_monitoring_dashboard" "app_metrics" {
  project = var.project_id
  dashboard_json = jsonencode({
    displayName = "fastFSO Application Metrics (${var.environment})"
    gridLayout = {
      columns = 2
      widgets = [
        # ── Row 1: HTTP Request Rate & Latency ──────────────────────────
        {
          title = "Request Rate by Route"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.label.\"http_route\""]
                    }
                  }
                }
                plotType = "LINE"
              },
            ]
            yAxis = { label = "req/s" }
          }
        },
        {
          title = "Request Latency p50 / p95 / p99"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_50"
                      crossSeriesReducer = "REDUCE_MEAN"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "p50"
              },
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_MEAN"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "p95"
              },
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_99"
                      crossSeriesReducer = "REDUCE_MEAN"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "p99"
              },
            ]
            yAxis = { label = "ms" }
          }
        },

        # ── Row 2: Error Rate & DB Query Latency ────────────────────────
        {
          title = "Error Rate (4xx / 5xx)"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\" metric.label.\"status_code\">=400 metric.label.\"status_code\"<500"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "4xx"
              },
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/http.server.request.duration\" resource.type=\"generic_task\" metric.label.\"status_code\">=500"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "5xx"
              },
            ]
            yAxis = { label = "errors/s" }
          }
        },
        {
          title = "DB Query Latency by Name"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/db.query.duration\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_MEAN"
                      groupByFields      = ["metric.label.\"query_name\""]
                    }
                  }
                }
                plotType = "LINE"
              },
            ]
            yAxis = { label = "ms" }
          }
        },

        # ── Row 3: DB Connection Pool & Auth ─────────────────────────────
        {
          title = "DB Connection Pool Usage"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/db.pool.connections\" resource.type=\"generic_task\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_MEAN"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.label.\"state\""]
                    }
                  }
                }
                plotType = "STACKED_AREA"
              },
            ]
            yAxis = { label = "connections" }
          }
        },
        {
          title = "Auth Login Success / Failure Rate"
          xyChart = {
            dataSets = [
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/auth.login.total\" resource.type=\"generic_task\" metric.label.\"result\"=\"success\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "success"
              },
              {
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"workload.googleapis.com/auth.login.total\" resource.type=\"generic_task\" metric.label.\"result\"=\"failure\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = []
                    }
                  }
                }
                plotType       = "LINE"
                legendTemplate = "failure"
              },
            ]
            yAxis = { label = "logins/s" }
          }
        },
      ]
    }
  })
}
