package tests

import "go.mongodb.org/mongo-driver/bson/primitive"

//mongowrapper:aggregation users
type UserView struct {
	Id      primitive.ObjectID `bson:"_id"`
	Email   string             `bson:"email"`
	Profile Profile            `bson:"profile"`
}

//mongowrapper:collection users
type User struct {
	Id      primitive.ObjectID `bson:"_id,omitempty"`
	Email   string             `bson:"email"`
	Profile Profile            `bson:"profile"`
	Address struct {
		City string
	} `bson:"address"`
	Fin         *Finance
	Permissions map[string]interface{}
	Ids         map[string]int
}

type Profile struct {
	FirstName string `bson:"first_name"`
	LastName  string `bson:"last_name"`
}

type Finance struct {
	Income int64
}

type flag struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
