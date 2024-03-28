package templates

import (
	"strings"
)

var MultipleAppTemplates = strings.ReplaceAll(strings.TrimSpace(`
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/app"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/auth"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/dsl"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/impl"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/requests"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/responses"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Position struct {
	Scope string #bson:"scope" json:"scope" validate:"required"#
	Map   int32  #bson:"map" json:"map" validate:"gte=0"#
	X     uint16 #bson:"x" json:"x"#
	Y     uint16 #bson:"y" json:"y"#
}

type Character struct {
	ID          primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	AccountID   primitive.ObjectID #bson:"account_id" json:"account_id" validate:"required"#
	DisplayName string             #bson:"display_name" json:"display_name" validate:"required"#
	Position    Position           #bson:"position" json:"position" validate:"dive"#
}

type Account struct {
	ID       primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	Login    string             #bson:"login" json:"login" validate:"required"#
	Password string             #bson:"password" json:"password" validate:"required"#
}

type Scope struct {
	ID          primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	Key         string             #bson:"key" json:"key" validate:"required"#
	TemplateKey string             #bson:"template_key" json:"template_key"#
}

type Map struct {
	ID      primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	ScopeID primitive.ObjectID #bson:"scope_id" json:"scope_id" validate:"required"#
	Index   int32              #bson:"index" json:"index" validate:"gte=0"#
	Drop    [][][]uint32       #bson:"drop" json:"drop"#
}

func LaunchServer() {
	host, _ := os.LookupEnv("DB_HOST")
	port, _ := os.LookupEnv("DB_PORT")
	username, _ := os.LookupEnv("DB_USER")
	password, _ := os.LookupEnv("DB_PASS")
	apiKey, ok := os.LookupEnv("SERVER_API_KEY")
	apiKey = strings.TrimSpace(apiKey)
	if !ok || apiKey == "" {
		panic("missing api key")
	}

	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	portValue, err := strconv.ParseUint(strings.TrimSpace(port), 10, 16)
	if err != nil {
		panic("invalid port")
	}

	settings := &dsl.Settings{
		Debug: true,
		Connection: dsl.Connection{
			Args: dsl.ConnectionFields{
				Host:     host,
				Port:     uint16(portValue),
				Username: username,
				Password: password,
			},
		},
		Global: dsl.Global{
			ListMaxResults: 20,
		},
		Auth: dsl.Auth{
			TableRef: dsl.TableRef{
				Db:         "auth-db-multichar",
				Collection: "api-keys",
			},
		},
		Resources: map[string]dsl.Resource{
			"accounts": {
				TableRef: dsl.TableRef{
					Db:         "universe-multichar",
					Collection: "accounts",
				},
				Type:       dsl.ListResource,
				Projection: bson.M{"login": 1, "password": 1},
				Methods: map[string]dsl.ResourceMethod{
					"by-login": {
						Type: dsl.View,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							login := ""
							(echo.QueryParamsBinder(context)).String("login", &login)
							login = strings.TrimSpace(login)
							if login == "" {
								return context.JSON(http.StatusBadRequest, echo.Map{
									"code": "missing-lookup",
								})
							}

							filter_ := map[string]any{}
							maps.Copy(filter_, filter)
							filter_["login"] = login
							v := Account{}
							if success, err := impl.GetDocument(context, collection.FindOne(context.Request().Context(), filter_), &v); success {
								return responses.OkWith(context, v)
							} else {
								return err
							}
						},
					},
				},
				ModelType:      dsl.ModelType[Account],
				SoftDelete:     true,
				ListMaxResults: 20,
				Indexes: map[string]dsl.Index{
					"unique-login": {
						Unique: true,
						Fields: []string{"login"},
					},
				},
			},
			"characters": {
				TableRef: dsl.TableRef{
					Db:         "universe-multichar",
					Collection: "characters",
				},
				Type:       dsl.ListResource,
				Projection: bson.M{"display_name": 1, "password": 1},
				Methods: map[string]dsl.ResourceMethod{
					"by-account": {
						Type: dsl.View,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							binder := echo.QueryParamsBinder(context)
							login := ""
							binder.String("login", &login)
							login = strings.TrimSpace(login)
							filter_ := bson.M{}
							maps.Copy(filter_, filter)
							filter_["_deleted"] = bson.M{"$ne": true}
							if login != "" {
								filter_["login"] = login
							} else {
								id := ""
								binder.String("id", &id)
								if id == "" {
									return context.JSON(http.StatusBadRequest, echo.Map{
										"code": "missing-lookup",
									})
								}
								if objId, err := primitive.ObjectIDFromHex(id); err != nil {
									return context.JSON(http.StatusBadRequest, echo.Map{
										"code": "bad-lookup",
									})
								} else {
									filter_["_id"] = objId
								}
							}

							ctx := context.Request().Context()

							// First, retrieve the account.
							v := Account{}
							if success, err := impl.GetDocument(context, collection.Database().Collection("accounts").FindOne(ctx, filter_), &v); !success {
								return err
							}

							// Then, retrieve the characters.
							if cursor, err := collection.Find(ctx, bson.M{"account_id": v.ID, "_deleted": bson.M{"$ne": true}}); err != nil {
								return responses.InternalError(context)
							} else {
								characters := []Character{}
								if success, err := impl.GetDocuments[Character](context, cursor, &characters); !success {
									return err
								} else {
									return responses.OkWith(context, characters)
								}
							}
						},
					},
				},
				ModelType:      dsl.ModelType[Character],
				SoftDelete:     true,
				ListMaxResults: 20,
				Indexes: map[string]dsl.Index{
					"unique-login": {
						Unique: true,
						Fields: []string{"login"},
					},
					"unique-nickname": {
						Unique: true,
						Fields: []string{"display_name"},
					},
				},
			},
			"scopes": {
				Type: dsl.ListResource,
				TableRef: dsl.TableRef{
					Db:         "universe-multichar",
					Collection: "scopes",
				},
				ModelType:  dsl.ModelType[Scope],
				SoftDelete: true,
				Projection: bson.M{"key": 1, "template_key": 1},
				Indexes: map[string]dsl.Index{
					"key": {
						Unique: true,
						Fields: []string{"key"},
					},
				},
			},
			"maps": {
				Type: dsl.ListResource,
				TableRef: dsl.TableRef{
					Db:         "universe-multichar",
					Collection: "maps",
				},
				ModelType:  dsl.ModelType[Map],
				SoftDelete: true,
				Projection: bson.M{"scope_id": 1, "index": 1},
				Indexes: map[string]dsl.Index{
					"unique-key": {
						Unique: true,
						Fields: []string{"scope_id", "index"},
					},
					"scope": {
						Unique: false,
						Fields: []string{"scope_id"},
					},
				},
				Methods: map[string]dsl.ResourceMethod{
					"by-scope": {
						Type: dsl.View,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							scope := ""
							id := ""
							binder := echo.QueryParamsBinder(context)
							binder.String("scope", &scope)
							filter_ := bson.M{}
							maps.Copy(filter_, filter)
							filter_["_deleted"] = bson.M{"$ne": true}
							if scope != "" {
								filter_["key"] = scope
							} else {
								binder.String("id", &id)
								if id == "" {
									return context.JSON(http.StatusBadRequest, echo.Map{
										"code": "missing-lookup",
									})
								}
								if objId, err := primitive.ObjectIDFromHex(id); err != nil {
									return context.JSON(http.StatusBadRequest, echo.Map{
										"code": "bad-lookup",
									})
								} else {
									filter_["_id"] = objId
								}
							}

							ctx := context.Request().Context()

							// First, retrieve the scope.
							var scopeId primitive.ObjectID
							var scopeDoc Scope
							if success, err := impl.GetDocument(context, collection.Database().Collection("scopes").FindOne(ctx, filter_), &scopeDoc); !success {
								return err
							} else {
								scopeId = scopeDoc.ID
							}

							// For a given/retrieved scope, retrieve the maps.
							if result, err := collection.Find(ctx, bson.M{"_deleted": bson.M{"$ne": true}, "scope_id": scopeId}); err != nil {
								return responses.InternalError(context)
							} else {
								items := []Map{}
								if success, err := impl.GetDocuments[Map](context, result, &items); !success {
									return err
								} else {
									return responses.OkWith(context, items)
								}
							}
						},
					},
				},
				ItemMethods: map[string]dsl.ItemMethod{
					"set-drop": {
						Type: dsl.Operation,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M, id primitive.ObjectID) error {
							ctx := context.Request().Context()
							var body struct {
								Drops [][][]uint32 #json:"drops"#
								From  int32        #json:"from"#
							}
							if success, err := requests.ReadJSONBody(context, nil, &body); !success {
								return err
							}
							if body.From < 0 {
								return context.JSON(http.StatusBadRequest, echo.Map{"code": "invalid-from"})
							}
							filter_ := bson.M{}
							maps.Copy(filter_, filter)
							filter_["_deleted"] = bson.M{"$ne": true}
							filter_["_id"] = id
							var map_ Map
							if success, err := impl.GetDocument(context, collection.FindOne(ctx, filter_), &map_); !success {
								return err
							}

							// The final computed drop.
							var drop = map_.Drop
							if drop == nil {
								drop = [][][]uint32{}
							}
							// The size of the drop segment to add.
							var newDropLength = len(body.Drops)
							// The size of the current drop.
							var dropLength = len(map_.Drop)
							// The lastIndex+1 of the new drop, once inserted.
							var from_ = int(body.From)
							var newDropFinalIndex = int(body.From) + newDropLength
							// Re-allocate a new array if it would result
							// in a bigger one.
							if newDropFinalIndex > dropLength {
								drop = make([][][]uint32, newDropFinalIndex)
								for i := 0; i < dropLength; i++ {
									drop[i] = map_.Drop[i]
								}
							}
							// Then, map the new elements.
							for i := 0; i < newDropLength; i++ {
								drop[i+from_] = body.Drops[i]
							}

							if _, err := collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"drop": drop}}); err != nil {
								return err
							} else {
								return responses.Ok(context)
							}
						},
					},
				},
			},
		},
	}

	if application, err := app.MakeServer(settings, nil, func(client *mongo.Client, settings *dsl.Settings) {
		ctx := context.Background()

		// First, know whether a setup already occurred.
		lifecycleCollection := client.Database("lifecycle-multichar").Collection("setup")
		var result bson.M
		if err := lifecycleCollection.FindOne(ctx, bson.M{}).Decode(&result); err != nil {
			// Checking whether an error occurred or trying to make
			// a brand-new setup.
			if !errors.Is(err, mongo.ErrNoDocuments) {
				panic(fmt.Sprintf("error retrieving initial setup: %s", err))
			} else if _, err := lifecycleCollection.InsertOne(ctx, bson.M{"done": true}); err != nil {
				panic(fmt.Sprintf("error doing initial setup: %s", err))
			}
		} else {
			// Setup is already done by this point.
			return
		}

		// Then, inserting the key.
		slog.Info("Initializing default key...")
		authCollection := client.Database(settings.Auth.Db).Collection(settings.Auth.Collection)
		if _, err := authCollection.InsertOne(ctx, &auth.AuthToken{
			ApiKey:     apiKey,
			ValidUntil: nil,
			Permissions: bson.M{
				"*": bson.A{"read", "write", "delete"},
			},
		}); err != nil {
			panic(fmt.Sprintf("error installing the setup: %s", err))
		}

		scopeWithMaps := map[string]int32{
			// Populate this structure with your static scopes and maps.
			// This is per-game configuration.
		}
		scopesCollection := client.Database("universe-multichar").Collection("scopes")
		mapsCollection := client.Database("universe-multichar").Collection("maps")
		slog.Info("Initializing scopes...")
		for scope, maps_ := range scopeWithMaps {
			slog.Info(fmt.Sprintf("Initializing scope %s and their maps...", scope))
			if result, err := scopesCollection.InsertOne(ctx, &Scope{
				Key: scope, TemplateKey: "",
			}); err != nil {
				panic(fmt.Sprintf("error installing static scope %s: %s", scope, err))
			} else {
				mapDocs := make([]any, maps_)
				var index int32
				for index = 0; index < maps_; index++ {
					mapDocs[index] = Map{
						ScopeID: result.InsertedID.(primitive.ObjectID),
						Index:   index,
						Drop:    make([][][]uint32, 0),
					}
				}
				if _, err := mapsCollection.InsertMany(ctx, mapDocs); err != nil {
					panic(fmt.Sprintf("error installing %d maps for scope: %s", maps_, scope))
				}
			}
		}
	}); err != nil {
		// Remember this is an example.
		slog.Error("An error has occurred: " + err.Error())
	} else {
		// It will panic only on error.
		if err := application.Run("0.0.0.0:80"); err != nil {
			slog.Error("An error has occurred: " + err.Error())
            os.Exit(1)
		}
	}
}

func main() {
	LaunchServer()
}
`), "#", "`")
