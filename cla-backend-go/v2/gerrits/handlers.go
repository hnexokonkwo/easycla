package gerrits

import (
	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/gerrits"
	v1Gerrits "github.com/communitybridge/easycla/cla-backend-go/gerrits"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

type ProjectService interface { //nolint
	GetProjectByID(projectID string) (*v1Models.Project, error)
}

// Configure the gerrit api
func Configure(api *operations.EasyclaAPI, service v1Gerrits.Service, projectService ProjectService, eventService events.Service) {
	api.GerritsDeleteGerritHandler = gerrits.DeleteGerritHandlerFunc(
		func(params gerrits.DeleteGerritParams, authUser *auth.User) middleware.Responder {
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			projectModel, err := projectService.GetProjectByID(params.ProjectID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithPayload(errorResponse(err))
			}
			// verify user have access to the project
			if !authUser.IsUserAuthorized(auth.Project, projectModel.ProjectExternalID) {
				return gerrits.NewDeleteGerritUnauthorized()
			}
			gerrit, err := service.GetGerrit(params.GerritID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithPayload(errorResponse(err))
			}
			// verify gerrit project is same as the request
			if gerrit.ProjectID != params.ProjectID {
				return gerrits.NewDeleteGerritBadRequest().WithPayload(&models.ErrorResponse{
					Code:    "400",
					Message: "provided project id does not match with gerrit project id",
				})
			}
			// delete the gerrit
			err = service.DeleteGerrit(params.GerritID)
			if err != nil {
				return gerrits.NewDeleteGerritBadRequest().WithPayload(errorResponse(err))
			}
			// record the event
			eventService.LogEvent(&events.LogEventArgs{
				EventType:    events.GerritRepositoryDeleted,
				ProjectModel: projectModel,
				LfUsername:   authUser.UserName,
				EventData: &events.GerritDeletedEventData{
					GerritRepositoryName: gerrit.GerritName,
				},
			})
			return gerrits.NewDeleteGerritOK()
		})

	api.GerritsAddGerritHandler = gerrits.AddGerritHandlerFunc(
		func(params gerrits.AddGerritParams, authUser *auth.User) middleware.Responder {
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			projectModel, err := projectService.GetProjectByID(params.ProjectID)
			if err != nil {
				return gerrits.NewAddGerritBadRequest().WithPayload(errorResponse(err))
			}
			// verify user have access to the project
			if !authUser.IsUserAuthorized(auth.Project, projectModel.ProjectExternalID) {
				return gerrits.NewDeleteGerritUnauthorized()
			}
			// add the gerrit
			addGerritInput := &v1Models.AddGerritInput{
				GerritName:  params.AddGerritInput.GerritName,
				GerritURL:   params.AddGerritInput.GerritURL,
				GroupIDCcla: params.AddGerritInput.GroupIDCcla,
				GroupIDIcla: params.AddGerritInput.GroupIDIcla,
			}
			result, err := service.AddGerrit(params.ProjectID, addGerritInput)
			if err != nil {
				return gerrits.NewAddGerritBadRequest().WithPayload(errorResponse(err))
			}
			// record the event
			eventService.LogEvent(&events.LogEventArgs{
				EventType:    events.GerritRepositoryAdded,
				ProjectModel: projectModel,
				LfUsername:   authUser.UserName,
				EventData: &events.GerritAddedEventData{
					GerritRepositoryName: utils.StringValue(params.AddGerritInput.GerritName),
				},
			})
			return gerrits.NewAddGerritOK().WithPayload(*result)
		})
}

type codedResponse interface {
	Code() string
}

func errorResponse(err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}

	return &e
}
