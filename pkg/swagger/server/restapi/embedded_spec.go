// Code generated by go-swagger; DO NOT EDIT.

package restapi

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
)

var (
	// SwaggerJSON embedded version of the swagger document used at generation time
	SwaggerJSON json.RawMessage
	// FlatSwaggerJSON embedded flattened version of the swagger document used at generation time
	FlatSwaggerJSON json.RawMessage
)

func init() {
	SwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "HTTP server in Go with Swagger endpoints definition.",
    "title": "go-rest-api",
    "version": "0.1.0"
  },
  "paths": {
    "/alive": {
      "get": {
        "produces": [
          "text/plain"
        ],
        "operationId": "checkHealth",
        "responses": {
          "200": {
            "description": "OK message.",
            "schema": {
              "type": "string",
              "enum": [
                "Yes"
              ]
            }
          }
        }
      }
    },
    "/api/report": {
      "get": {
        "description": "Get statistics report for given dates",
        "parameters": [
          {
            "type": "string",
            "format": "date-time",
            "description": "Recorded stats from a given date. Default is today minus 1 year.",
            "name": "dateFrom",
            "in": "query"
          },
          {
            "type": "string",
            "format": "date-time",
            "description": "Recorded stats to given date. Default is today.",
            "name": "dateTo",
            "in": "query"
          }
        ],
        "responses": {
          "200": {
            "description": "Returns the report of a given day",
            "schema": {
              "$ref": "#/definitions/GitStatisticsReport"
            }
          },
          "404": {
            "description": "Not any reports found"
          }
        }
      }
    },
    "/hello/{user}": {
      "get": {
        "description": "Returns a greeting to the user!",
        "parameters": [
          {
            "type": "string",
            "description": "The name of the user to greet.",
            "name": "user",
            "in": "path",
            "required": true
          }
        ],
        "responses": {
          "200": {
            "description": "Returns the greeting.",
            "schema": {
              "type": "string"
            }
          },
          "400": {
            "description": "Invalid characters in \"user\" were provided."
          }
        }
      }
    }
  },
  "definitions": {
    "GitAuthorContributions": {
      "properties": {
        "additions": {
          "type": "integer"
        },
        "deletions": {
          "type": "integer"
        }
      }
    },
    "GitCommits": {
      "properties": {
        "contributions": {
          "type": "object",
          "$ref": "#/definitions/GitAuthorContributions"
        },
        "count": {
          "type": "integer"
        },
        "datetime": {
          "type": "string",
          "format": "date-time"
        },
        "username": {
          "type": "string"
        }
      }
    },
    "GitStatisticsReport": {
      "type": "object",
      "properties": {
        "commits": {
          "type": "array",
          "items": {
            "$ref": "#/definitions/GitCommits"
          }
        },
        "dateFrom": {
          "type": "string",
          "format": "date-time"
        },
        "dateTo": {
          "type": "string",
          "format": "date-time"
        }
      }
    }
  }
}`))
	FlatSwaggerJSON = json.RawMessage([]byte(`{
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "HTTP server in Go with Swagger endpoints definition.",
    "title": "go-rest-api",
    "version": "0.1.0"
  },
  "paths": {
    "/alive": {
      "get": {
        "produces": [
          "text/plain"
        ],
        "operationId": "checkHealth",
        "responses": {
          "200": {
            "description": "OK message.",
            "schema": {
              "type": "string",
              "enum": [
                "Yes"
              ]
            }
          }
        }
      }
    },
    "/api/report": {
      "get": {
        "description": "Get statistics report for given dates",
        "parameters": [
          {
            "type": "string",
            "format": "date-time",
            "description": "Recorded stats from a given date. Default is today minus 1 year.",
            "name": "dateFrom",
            "in": "query"
          },
          {
            "type": "string",
            "format": "date-time",
            "description": "Recorded stats to given date. Default is today.",
            "name": "dateTo",
            "in": "query"
          }
        ],
        "responses": {
          "200": {
            "description": "Returns the report of a given day",
            "schema": {
              "$ref": "#/definitions/GitStatisticsReport"
            }
          },
          "404": {
            "description": "Not any reports found"
          }
        }
      }
    },
    "/hello/{user}": {
      "get": {
        "description": "Returns a greeting to the user!",
        "parameters": [
          {
            "type": "string",
            "description": "The name of the user to greet.",
            "name": "user",
            "in": "path",
            "required": true
          }
        ],
        "responses": {
          "200": {
            "description": "Returns the greeting.",
            "schema": {
              "type": "string"
            }
          },
          "400": {
            "description": "Invalid characters in \"user\" were provided."
          }
        }
      }
    }
  },
  "definitions": {
    "GitAuthorContributions": {
      "properties": {
        "additions": {
          "type": "integer"
        },
        "deletions": {
          "type": "integer"
        }
      }
    },
    "GitCommits": {
      "properties": {
        "contributions": {
          "type": "object",
          "$ref": "#/definitions/GitAuthorContributions"
        },
        "count": {
          "type": "integer"
        },
        "datetime": {
          "type": "string",
          "format": "date-time"
        },
        "username": {
          "type": "string"
        }
      }
    },
    "GitStatisticsReport": {
      "type": "object",
      "properties": {
        "commits": {
          "type": "array",
          "items": {
            "$ref": "#/definitions/GitCommits"
          }
        },
        "dateFrom": {
          "type": "string",
          "format": "date-time"
        },
        "dateTo": {
          "type": "string",
          "format": "date-time"
        }
      }
    }
  }
}`))
}
