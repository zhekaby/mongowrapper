package main

var writerAggregation = `
import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

{{range .Aggregations}}
type ArrayOf{{ .Typ }} []*{{ .Typ }}

func (s *{{ .Name }}Repository) AggregateTo{{ .Typ }}(ctx context.Context, pipeline mongo.Pipeline, limit int)  ([]*{{ .Typ }}, error) {
	if cursor, err := s.c.Aggregate(ctx, pipeline); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		} else {
			return nil, err
		}
	}else {
		records := make([]*{{ .Typ }}, 0, limit)
		for cursor.Next(ctx) {
			t := {{ .Typ }}{}
			err := cursor.Decode(&t)
			if err != nil {
				return records, err
			}
			records = append(records, &t)
		}
		return records, nil
	}
}

{{ if $.HasId }}
func (v{{ .Typ }} *{{ .Typ }}) GetId() primitive.ObjectID {
	return v{{ .Typ }}.{{ $.IdGoPath }}
}

func (entities ArrayOf{{ .Typ }}) AsIdAware() []IdAware {
	res := make([]IdAware, len(entities))
	for i, v := range entities {
		res[i] = v
	}
	return res
}

func (entities ArrayOf{{ .Typ }}) AsLookupById() map[primitive.ObjectID]*{{ .Typ }} {
	ids := make(map[primitive.ObjectID]*{{ .Typ }}, len(entities))
	for _, e := range entities {
		ids[e.{{ $.IdGoPath }}] = e
	}
	return ids
}
{{end}}

{{end}}
`
