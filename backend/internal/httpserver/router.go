package httpserver

import (
	"net/http"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/integrations/extapi"
	"orenkod/backend/internal/oidc"
	"orenkod/backend/internal/rules"
)

type Routes struct {
	Health            http.HandlerFunc
	Dashboard         http.HandlerFunc
	Competitions      http.HandlerFunc
	CreateCompetition http.HandlerFunc
	Athletes          http.HandlerFunc
	Rating            http.HandlerFunc
	Team              http.HandlerFunc
	Notifications     http.HandlerFunc
	TelegramStatus    http.HandlerFunc
	Auth              *auth.Service
	OIDC              *oidc.Provider
	ExtAPI            *extapi.Service
	Extra             func(mux *http.ServeMux, a *auth.Service)
	Platform          PlatformRoutes
}

type PlatformRoutes struct {
	Login, Register, SendCode, Logout, Me, Switch, CreateCompetition                                                                    http.HandlerFunc
	Catalogs, PublicHome, Verify                                                                                              http.HandlerFunc
	Publish, Apply, Applications, DecideApp                                                                                   http.HandlerFunc
	RequestLink, DecideLink, Links                                                                                            http.HandlerFunc
	Judges, AssignJudge, Assignments                                                                                          http.HandlerFunc
	SaveResults, Protocol, History                                                                                            http.HandlerFunc
	RankApply, RankList, RankDecide                                                                                           http.HandlerFunc
	Participants http.HandlerFunc
	CatApply, CatList, CatDecide                                                                                              http.HandlerFunc
	TeamSave, TeamOrder                                                                                                       http.HandlerFunc
	Enrollment, AthleteReport, RegistryXLSX, AddType, Development, Audit, Certificate, Inbox, Rating, TeamGet, ApproveAthlete http.HandlerFunc
	TelegramLinkCode, TelegramLinkStatus, TelegramUnlink                                                                      http.HandlerFunc
}

func NewRouter(routes Routes) http.Handler {
	mux := http.NewServeMux()
	register(mux, "GET /api/health", routes.Health)
	register(mux, "GET /api/public/home", routes.Platform.PublicHome)
	register(mux, "GET /api/public/catalogs", routes.Platform.Catalogs)
	register(mux, "GET /api/verify/{code}", routes.Platform.Verify)
	register(mux, "POST /api/auth/login", routes.Platform.Login)
	register(mux, "POST /api/auth/register", routes.Platform.Register)
	register(mux, "POST /api/auth/send-code", routes.Platform.SendCode)
	register(mux, "GET /api/integrations/telegram/status", routes.TelegramStatus)

	if routes.Auth != nil {
		anyUser := routes.Auth.Middleware()
		fed := routes.Auth.Middleware(rules.RoleFederation)
		fedCoach := routes.Auth.Middleware(rules.RoleFederation, rules.RoleCoach)
		coach := routes.Auth.Middleware(rules.RoleCoach)
		athlete := routes.Auth.Middleware(rules.RoleAthlete)
		judge := routes.Auth.Middleware(rules.RoleJudge)
		fedOrJudge := routes.Auth.Middleware(rules.RoleFederation, rules.RoleJudge)
		fedCoachAthlete := routes.Auth.Middleware(rules.RoleFederation, rules.RoleCoach, rules.RoleAthlete)
		fedMinistry := routes.Auth.Middleware(rules.RoleFederation, rules.RoleMinistry)
		coachAthlete := routes.Auth.Middleware(rules.RoleCoach, rules.RoleAthlete)

		register(mux, "POST /api/auth/logout", anyUser(routes.Platform.Logout))
		register(mux, "GET /api/auth/me", anyUser(routes.Platform.Me))
		register(mux, "GET /api/integrations/telegram/link-status", anyUser(routes.Platform.TelegramLinkStatus))
		register(mux, "POST /api/integrations/telegram/link-code", anyUser(routes.Platform.TelegramLinkCode))
		register(mux, "DELETE /api/integrations/telegram/link", anyUser(routes.Platform.TelegramUnlink))
		register(mux, "POST /api/auth/switch-role", anyUser(routes.Platform.Switch))
		register(mux, "GET /api/dashboard", anyUser(routes.Dashboard))
		register(mux, "GET /api/competitions", anyUser(routes.Competitions))
		register(mux, "POST /api/competitions", fed(routes.Platform.CreateCompetition))
		register(mux, "POST /api/competitions/{id}/publish", fed(routes.Platform.Publish))
		register(mux, "POST /api/competitions/{id}/apply", coachAthlete(routes.Platform.Apply))
		register(mux, "GET /api/competitions/{id}/applications", fed(routes.Platform.Applications))
		register(mux, "GET /api/applications", fed(routes.Platform.Applications))
		register(mux, "POST /api/applications/{id}/decide", fed(routes.Platform.DecideApp))
		register(mux, "GET /api/athletes", fedCoach(routes.Athletes))
		register(mux, "POST /api/athletes/{id}/approve", fed(routes.Platform.ApproveAthlete))
		register(mux, "GET /api/rating", anyUser(routes.Platform.Rating))
		register(mux, "GET /api/team", anyUser(routes.Platform.TeamGet))
		register(mux, "POST /api/team", fed(routes.Platform.TeamSave))
		register(mux, "GET /api/team/order", fedMinistry(routes.Platform.TeamOrder))
		register(mux, "GET /api/notifications", anyUser(routes.Platform.Inbox))
		register(mux, "POST /api/coach-links", coach(routes.Platform.RequestLink))
		register(mux, "GET /api/coach-links", anyUser(routes.Platform.Links))
		register(mux, "POST /api/coach-links/{id}/decide", athlete(routes.Platform.DecideLink))
		register(mux, "GET /api/judges", routes.Auth.Middleware(rules.RoleFederation, rules.RoleMinistry, rules.RoleJudge)(routes.Platform.Judges))
		register(mux, "GET /api/competitions/{id}/participants", fedOrJudge(routes.Platform.Participants))
		register(mux, "POST /api/competitions/{id}/judges", fed(routes.Platform.AssignJudge))
		register(mux, "GET /api/assignments", fedOrJudge(routes.Platform.Assignments))
		register(mux, "POST /api/competitions/{id}/results", fedOrJudge(routes.Platform.SaveResults))
		register(mux, "GET /api/competitions/{id}/protocol", anyUser(routes.Platform.Protocol))
		register(mux, "GET /api/results", anyUser(routes.Platform.History))
		register(mux, "POST /api/ranks", coachAthlete(routes.Platform.RankApply))
		register(mux, "GET /api/ranks", fedCoachAthlete(routes.Platform.RankList))
		register(mux, "POST /api/ranks/{id}/decide", fed(routes.Platform.RankDecide))
		register(mux, "POST /api/categories", judge(routes.Platform.CatApply))
		register(mux, "GET /api/categories", routes.Auth.Middleware(rules.RoleFederation, rules.RoleJudge)(routes.Platform.CatList))
		register(mux, "POST /api/categories/{id}/decide", fed(routes.Platform.CatDecide))
		register(mux, "GET /api/documents/enrollment", fedMinistry(routes.Platform.Enrollment))
		register(mux, "GET /api/documents/registry.xlsx", fedMinistry(routes.Platform.RegistryXLSX))
		register(mux, "GET /api/documents/athletes/{id}", anyUser(routes.Platform.AthleteReport))
		register(mux, "POST /api/catalogs/types", fed(routes.Platform.AddType))
		register(mux, "GET /api/development", fedMinistry(routes.Platform.Development))
		register(mux, "GET /api/audit", fed(routes.Platform.Audit))
		register(mux, "POST /api/certificates", athlete(routes.Platform.Certificate))
		if routes.OIDC != nil {
			routes.OIDC.Mount(mux, anyUser, fed)
		}
		if routes.Extra != nil {
			routes.Extra(mux, routes.Auth)
		}
		if routes.ExtAPI != nil {
			routes.ExtAPI.Mount(mux, fed)
		}
	} else {
		register(mux, "GET /api/dashboard", routes.Dashboard)
		register(mux, "GET /api/competitions", routes.Competitions)
		register(mux, "POST /api/competitions", routes.CreateCompetition)
		register(mux, "GET /api/athletes", routes.Athletes)
		register(mux, "GET /api/rating", routes.Rating)
		register(mux, "GET /api/team", routes.Team)
		register(mux, "GET /api/notifications", routes.Notifications)
	}
	return CORS(mux)
}

func register(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	if handler == nil {
		return
	}
	mux.HandleFunc(pattern, handler)
}
