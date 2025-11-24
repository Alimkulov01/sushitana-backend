package postgres

import (
	categoryrepo "sushitana/pkg/repository/postgres/category_repo"
	clientRepo "sushitana/pkg/repository/postgres/client_repo"
	productRepo "sushitana/pkg/repository/postgres/product_repo"
	userRepo "sushitana/pkg/repository/postgres/users_repo"

	"go.uber.org/fx"
)

var Module = fx.Options(
	userRepo.Module,
	clientRepo.Module,
	categoryrepo.Module,
	productRepo.Module,
)
