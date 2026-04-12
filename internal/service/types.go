package service

type ProductFilter struct {
	MinPrice   *float64
	MaxPrice   *float64
	CategoryID *string
	NameSearch *string
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

type CreateProductInput struct {
	Name          string
	Description   *string
	Price         float64
	Currency      *string
	CategoryID    string
	StockQuantity int
}

type UpdateProductInput struct {
	Name          *string
	Description   *string
	Price         *float64
	Currency      *string
	CategoryID    *string
	StockQuantity *int
}

type CreateCategoryInput struct {
	Name string
}
