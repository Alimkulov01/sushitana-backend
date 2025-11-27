package filerepo

import (
	"context"
	"database/sql"
	"fmt"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"

	"go.uber.org/fx"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		Logger logger.Logger
		DB     db.Querier
	}

	Repo interface {
		Create(ctx context.Context, req structs.CreateImage) (structs.Image, error)
		GetById(ctx context.Context, req structs.ImagePrimaryKey) (structs.Image, error)
		Delete(ctx context.Context, req structs.ImagePrimaryKey) error
		GetImage(ctx context.Context, req structs.GetImageRequest) (structs.GetImagerespones, error)
		GetAll(ctx context.Context) (structs.GetImagerespones, error)
	}

	repo struct {
		logger logger.Logger
		db     db.Querier
	}
)

func New(p Params) Repo {
	return &repo{
		logger: p.Logger,
		db:     p.DB,
	}
}

func (r repo) Create(ctx context.Context, req structs.CreateImage) (structs.Image, error) {
	var (
		query = `
			INSERT INTO "images"(
				"image_type",
				"image"
			) VALUES ($1, $2) RETURNING "id"
		`
		id sql.NullInt64
	)
	err := r.db.QueryRow(ctx, query, req.ImageType, req.Image).Scan(
		&id,
	)
	if err != nil {
		return structs.Image{}, err
	}
	return r.GetById(ctx, structs.ImagePrimaryKey{Id: id.Int64})
}

func (r repo) GetById(ctx context.Context, req structs.ImagePrimaryKey) (structs.Image, error) {
	var (
		query = `
			SELECT 
				"id",
				"image_type",
				"image"
			FROM "images"
			WHERE "id" = $1
		`
		id         sql.NullInt64
		image_type sql.NullString
		image      sql.NullString
	)
	err := r.db.QueryRow(ctx, query, req.Id).Scan(
		&id,
		&image_type,
		&image,
	)
	if err != nil {
		return structs.Image{}, err
	}

	return structs.Image{
		Id:        id.Int64,
		ImageType: image_type.String,
		Image:     image.String,
	}, nil
}

func (r repo) Delete(ctx context.Context, req structs.ImagePrimaryKey) error {
	_, err := r.db.Exec(ctx, `DELETE FROM "images" WHERE "id" = $1`, req.Id)
	return err
}

func (r repo) GetImage(ctx context.Context, req structs.GetImageRequest) (structs.GetImagerespones, error) {
	var (
		query = `
			SELECT
				id,
				image_type,
				image
			FROM images
			WHERE (image_type ILIKE $1 OR CAST(id AS TEXT) ILIKE $2)
		`
		resp structs.GetImagerespones
	)

	arg1 := "%" + req.ImageType + "%"
	arg2 := "%" + fmt.Sprintf("%d", req.Id) + "%"

	rows, err := r.db.Query(ctx, query, arg1, arg2)
	if err != nil {
		return structs.GetImagerespones{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			image      structs.Image
			id         sql.NullInt64
			image_type sql.NullString
			image_name sql.NullString
		)

		err = rows.Scan(&id, &image_type, &image_name)
		if err != nil {
			return structs.GetImagerespones{}, err
		}

		image = structs.Image{
			Id:        id.Int64,
			ImageType: image_type.String,
			Image:     image_name.String,
		}
		resp.Images = append(resp.Images, image)
	}

	return resp, nil
}

func (r repo) GetAll(ctx context.Context) (structs.GetImagerespones, error) {
	var (
		query = `
			SELECT
				id,
				image_type,
				image
			FROM images
		`
		resp structs.GetImagerespones
	)

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return structs.GetImagerespones{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			image      structs.Image
			id         sql.NullInt64
			image_type sql.NullString
			image_name sql.NullString
		)

		err = rows.Scan(&id, &image_type, &image_name)
		if err != nil {
			return structs.GetImagerespones{}, err
		}

		image = structs.Image{
			Id:        id.Int64,
			ImageType: image_type.String,
			Image:     image_name.String,
		}
		resp.Images = append(resp.Images, image)
	}

	return resp, nil
}
