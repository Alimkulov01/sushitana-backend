package postgres

import (
	cartrepo "sushitana/pkg/repository/postgres/cart_repo"
	categoryrepo "sushitana/pkg/repository/postgres/category_repo"
	clientRepo "sushitana/pkg/repository/postgres/client_repo"
	employeerepo "sushitana/pkg/repository/postgres/employee_repo"
	filerepo "sushitana/pkg/repository/postgres/file_repo"
	productRepo "sushitana/pkg/repository/postgres/product_repo"
	rolerepo "sushitana/pkg/repository/postgres/role_repo"
	userRepo "sushitana/pkg/repository/postgres/users_repo"

	"go.uber.org/fx"
)

var Module = fx.Options(
	userRepo.Module,
	clientRepo.Module,
	categoryrepo.Module,
	productRepo.Module,
	filerepo.Module,
	rolerepo.Module,
	employeerepo.Module,
	cartrepo.Module,
)
