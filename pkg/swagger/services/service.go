package services

import (
	"log"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/svandecappelle/gitcontrib/pkg/swagger/server/models"
	"github.com/svandecappelle/gitcontrib/pkg/swagger/server/restapi"
	"github.com/svandecappelle/gitcontrib/pkg/swagger/server/restapi/operations"
)

func Start() error {
	// Initialize Swagger
	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		log.Fatalln(err)
	}

	api := operations.NewHelloAPIAPI(swaggerSpec)
	server := restapi.NewServer(api)

	defer func() {
		if err := server.Shutdown(); err != nil {
			// error handle
			log.Fatalln(err)
		}
	}()

	server.Port = 8080

	api.CheckHealthHandler = operations.CheckHealthHandlerFunc(Health)
	api.GetHelloUserHandler = operations.GetHelloUserHandlerFunc(GetHelloUser)
	api.GetAPIReportHandler = operations.GetAPIReportHandlerFunc(GetApiReport)

	// Start server which listening
	if err := server.Serve(); err != nil {
		log.Fatalln(err)
		return err
	}
	return nil
}

//Health route returns OK
func Health(operations.CheckHealthParams) middleware.Responder {
	return operations.NewCheckHealthOK().WithPayload("Yes")
}

//GetHelloUser returns Hello + your name
func GetHelloUser(user operations.GetHelloUserParams) middleware.Responder {
	return operations.NewGetHelloUserOK().WithPayload("Hello " + user.User + "!")
}

//GetApiReport returns a statistic report
func GetApiReport(params operations.GetAPIReportParams) middleware.Responder {
	var payload = models.GitStatisticsReport{}
	return operations.NewGetAPIReportOK().WithPayload(&payload)
}
