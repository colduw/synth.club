package main

import (
	"main/database"

	"github.com/joho/godotenv"
)

func main() {
	if loadErr := godotenv.Load("../../.env"); loadErr != nil {
		panic(loadErr)
	}

	database.SetupDatabase()

	cData := database.CHandle{
		Handle: "handle-goes-here",
		DID:    "did:plc:stuff",
		DHCode: "dh=stuff",
	}

	if createErr := database.Db().Model(&database.CHandle{}).Create(&cData).Error; createErr != nil {
		panic(createErr)
	}
}
