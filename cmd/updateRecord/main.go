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

	const handleToUpdate = "handle-goes-here"
	newcData := database.CHandle{
		Handle: "handle-goes-here",
		DID:    "did:plc:stuff",
		DHCode: "dh=stuff",
	}

	if updateErr := database.Db().Where("handle = ?", handleToUpdate).Updates(&newcData).Error; updateErr != nil {
		panic(updateErr)
	}
}
