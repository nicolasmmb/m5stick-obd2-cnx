package main

type TileKind uint8

const (
	tileNumber TileKind = iota
	tileTemperature
	tilePercent
	tileLambda
	tileVoltage
)

type Tile struct {
	PID   string
	Label string
	Unit  string
	Icon  int
	Kind  TileKind
}

type Page struct {
	Name  string
	Tiles []Tile
}

var pages = []Page{
	{Name: "STATUS"},
	{
		Name: "DADOS",
		Tiles: []Tile{
			{PID: "0D", Label: "VELOCIDADE", Unit: "km/h", Icon: iconGauge, Kind: tileNumber},
			{PID: "0C", Label: "RPM", Unit: "rpm", Icon: iconGauge, Kind: tileNumber},
			{PID: "2F", Label: "COMBUSTIVEL", Unit: "L", Icon: iconFuel, Kind: tilePercent},
		},
	},
	{
		Name: "TEMPERATURAS",
		Tiles: []Tile{
			{PID: "05", Label: "AGUA", Unit: "C", Icon: iconThermo, Kind: tileTemperature},
			{PID: "0F", Label: "AR ADM", Unit: "C", Icon: iconAir, Kind: tileTemperature},
			{PID: "3C", Label: "CATALISADOR", Unit: "C", Icon: iconCat, Kind: tileTemperature},
		},
	},
	{
		Name: "MOTOR",
		Tiles: []Tile{
			{PID: "04", Label: "CARGA", Unit: "%", Icon: iconGauge, Kind: tilePercent},
			{PID: "11", Label: "BORBOLETA", Unit: "%", Icon: iconRate, Kind: tilePercent},
			{PID: "44", Label: "LAMBDA", Unit: "lambda", Icon: iconAir, Kind: tileLambda},
		},
	},
	{
		Name: "DIAGNOSTICO",
		Tiles: []Tile{
			{PID: "42", Label: "TENSAO", Unit: "V", Icon: iconGauge, Kind: tileVoltage},
		},
	},
	{
		Name: "VIAGEM",
	},
}

var pageCount = len(pages)
