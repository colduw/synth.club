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

	const handleToDelete = "handle-goes-here"

	if deleteErr := database.Db().Unscoped().Delete(&database.CHandle{}, "handle = ?", handleToDelete).Error; deleteErr != nil {
		panic(deleteErr)
	}
}
