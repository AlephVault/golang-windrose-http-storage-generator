package templates

import "strings"

/**
TODO implementar estas pero como handlers en golang.

class GetMapsByScope(MethodHandler):
    """
    Get all the maps references inside a given scope.
    """

    def __call__(self, client: MongoClient, resource: str, method: str, db: str, collection: str, filter: dict):
        scope = request.args.get("scope")
        if scope:
            filter = {**filter, "key": scope}
        else:
            scope_id = request.args.get("id")
            if scope_id:
                filter = {**filter, "_id": ObjectId(scope_id)}
            else:
                return make_response(jsonify({"code": "missing-lookup"}), 400)
        document = client[db]['scopes'].find_one(filter)
        if document:
            result = list(client[db][collection].find({'_deleted': {}, 'scope_id': document['_id']},
                                                      ['index']))
            return make_response(jsonify(result), 200)
        else:
            return make_response(jsonify({"code": "not-found"}), 404)


class UpdateDrop(MethodHandler):
    """
    Updates part of the drop of a map. This update is done in linear slice.
    """

    def _replace(self, current_drop: list, from_idx: int, drops: list):
        """
        Ensures the current drop is updated with new drops.
        :param current_drop: The current drop(s) status.
        :param from_idx: The index to start updating from.
        :param drops: The drops to set.
        """

        current_drop_len = len(current_drop)
        to_idx = from_idx + len(drops)
        extra = to_idx - current_drop_len
        if extra > 0:
            current_drop.extend([[] for _ in range(extra)])
        current_drop[from_idx:to_idx] = drops

    def __call__(self, client: MongoClient, resource: str, method: str, db: str, collection: str, filter: dict):
        map_id = request.args.get("id")
        if map_id:
            filter = {**filter, "_id": ObjectId(map_id)}
        else:
            return make_response(jsonify({"code": "missing-lookup"}), 400)

        # Only allow JSON format.
        if not request.is_json:
            return make_response(jsonify({"code": "bad-format"}), 406)

        # Get the drops.
        drops = request.json.get("drops")
        if not isinstance(drops, list) or not all(isinstance(box, list) for box in drops):
            return make_response(jsonify({"code": "missing-or-invalid-drop"}), 400)

        # Get the index to apply the changes from.
        from_idx = request.json.get("from", 0)
        if not isinstance(from_idx, (int, float)) or from_idx < 0:
            return make_response(jsonify({"code": "bad-index"}), 400)
        from_idx = int(from_idx)

        # Get the map to change the drops.
        document = client[db][collection].find_one(filter)
        if document:
            # Get the drop, and update it.
            current_drop = document.get("drop", [])
            self._replace(current_drop, from_idx, [e or [] for e in drops])
            client[db][collection].update_one(filter, {"$set": {"drop": current_drop}})
            return make_response(jsonify({"code": "ok"}), 200)
        else:
            return make_response(jsonify({"code": "not-found"}), 404)

TODO también implementar esto en el callback de creación del server (hoy solo tengo lo de init default key):

    def _init_default_key(self, key: str):
        """
        A convenience utility to initialize an API key.
        :param key: The key to initialize.
        """

        LOGGER.info("Initializing default key...")
        self._client["auth-db"]["api-keys"].insert_one({"api-key": key})

    def _init_static_scopes(self, scopes: Dict[str, int]):
        """
        A convenience utility to initialize some static maps.
        :param scopes: The scopes keys to initialize, and their maps count.
        """

        LOGGER.info("Initializing scopes...")
        for scope, maps in scopes.items():
            LOGGER.info(f"Initializing scope {scope} and their {maps}...")
            scope_id = self._client["universe"]["scopes"].insert_one({
                "key": scope, "template_key": ""
            }).inserted_id
            self._client["universe"]["maps"].insert_many([
                {"scope_id": scope_id, "index": index, "drop": []}
                for index in range(max(0, maps))
            ])

    def __init__(self, import_name: str = __name__):
        super().__init__(self.SETTINGS, import_name=import_name)
        try:
            setup = self._client["lifecycle"]["setup"]
            result = setup.find_one()
            if not result:
                setup.insert_one({"done": True})
                self._init_default_key(os.environ['SERVER_API_KEY'])
                self._init_static_scopes({})
        except:
            pass
*/

var SimpleAppTemplate = strings.ReplaceAll(strings.TrimSpace(`
package main

import (
	"context"
	"errors"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/app"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/auth"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/dsl"
	"github.com/AlephVault/golang-standard-http-mongodb-storage/core/responses"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"maps"
	"os"
	"strconv"
	"strings"
)

type Position struct {
	Scope string #bson:"scope" json:"scope" validate:"required"#
	Map   int32  #bson:"map" json:"map" validate:"required,gte=0"#
	X     uint16 #bson:"x" json:"x"#
	Y     uint16 #bson:"y" json:"y"#
}

type Account struct {
	ID          primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	Login       string             #bson:"login" json:"login" validate:"required"#
	Password    string             #bson:"password" json:"password" validate:"required"#
	DisplayName string             #bson:"display_name" json:"display_name" validate:"required"#
	Position    Position           #bson:"position" json:"position" validate:"dive"#
}

type Scope struct {
	ID          primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	Key         string             #bson:"key" json:"key" validate:"required"#
	TemplateKey string             #bson:"template_key" json:"template_key" validate:"required"#
}

type Map struct {
	ID      primitive.ObjectID #bson:"_id,omitempty" json:"_id,omitempty"#
	ScopeID primitive.ObjectID #bson:"scope_id" json:"scope_id" validate:"required"#
	Index   int32              #bson:"index" json:"index" validate:"required,gte=0"#
	Drop    [][]uint32         #bson:"drop" json:"drop"#
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
				Db:         "auth-db",
				Collection: "api-keys",
			},
		},
		Resources: map[string]dsl.Resource{
			"accounts": {
				TableRef: dsl.TableRef{
					Db:         "universe-simple",
					Collection: "accounts",
				},
				Type:       dsl.ListResource,
				Projection: bson.M{"login": 1, "password": 1, "display_name": 1, "position": 1},
				Methods: map[string]dsl.ResourceMethod{
					"by-login": {
						Type: dsl.View,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							login := ""
							(echo.QueryParamsBinder(context)).String("login", &login)
							login = strings.TrimSpace(login)
							if login == "" {
								return context.JSON(400, map[string]any{
									"code": "missing-lookup",
								})
							}

							filter_ := map[string]any{}
							maps.Copy(filter_, filter)
							filter_["login"] = login
							var account Account
							if err := collection.FindOne(context.Request().Context(), filter_).Decode(&account); err != nil {
								if errors.Is(err, mongo.ErrNoDocuments) {
									return responses.NotFound(context)
								} else {
									return responses.InternalError(context)
								}
							} else {
								return responses.OkWith(context, account)
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
					"unique-nickname": {
						Unique: true,
						Fields: []string{"display_name"},
					},
				},
			},
			"scopes": {
				Type: dsl.ListResource,
				TableRef: dsl.TableRef{
					Db:         "universe-simple",
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
					Db:         "universe-simple",
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
				},
				Methods: map[string]dsl.ResourceMethod{
					"set-drop": {
						Type: dsl.Operation,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							// TODO implement.
							return nil
						},
					},
					"by-scope": {
						Type: dsl.View,
						Handler: func(context echo.Context, client *mongo.Client, resource, method string, collection *mongo.Collection, validatorMaker func() *validator.Validate, filter bson.M) error {
							// TODO implement.
							return nil
						},
					},
				},
			},
		},
	}

	if application, err := app.MakeServer(settings, nil, func(client *mongo.Client, settings *dsl.Settings) {
		ctx := context.Background()

		// First, know whether a setup already occurred.
		lifecycleCollection := client.Database("lifecycle").Collection("setup")
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
		}
		scopesCollection := client.Database("universe-simple").Collection("scopes")
		mapsCollection := client.Database("universe-simple").Collection("maps")
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
						Drop:    make([][]uint32, 0),
					}
				}
				if _, err := mapsCollection.InsertMany(ctx, mapDocs); err != nil {
					panic(fmt.Sprintf("error installing %d maps for scope: %s", maps_, scope))
				}
			}

		}

		// TODO add static scopes / maps insertion here.
	}); err != nil {
		// Remember this is an example.
		panic(err)
	} else {
		// It will panic only on error.
		panic(application.Run("0.0.0.0:80"))
	}
}

func main() {
	LaunchServer()
}
`), "#", "`")
