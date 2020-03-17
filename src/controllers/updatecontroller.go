package controllers

import (
	"fmt"
	"github.com/ventuary-lab/cache-updater/src/entities"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type UpdateController struct {
	DbDelegate *DbController
	ScDelegate *ShareController
}

func (uc *UpdateController) GrabAllAddressData () ([]byte, error) {
	dAppAddress := os.Getenv("DAPP_ADDRESS")
	nodeUrl := os.Getenv("NODE_URL")
	connectionUrl := nodeUrl + "/addresses/data/" + dAppAddress
	response, err := http.Get(connectionUrl)

	if err != nil {
		fmt.Println(err)
		return make([]byte, 0), err
	}

	defer response.Body.Close()

	byteValue, _ := ioutil.ReadAll(response.Body)

	return byteValue, nil
}

func (uc *UpdateController) UpdateStateChangedData (
	minHeight, maxHeight uint64,
) {
	blocks := entities.FetchBlocksRange(
		fmt.Sprintf("%v", minHeight),
		fmt.Sprintf("%v", maxHeight),
	)

	delimiter := "_"

	for _, block := range *blocks {
		blockWithTxList := entities.FetchTransactionsOnSpecificBlock(
			fmt.Sprintf("%v", *block.Height),
		)

		// Invoke Script Transaction: 16
		for _, tx := range blockWithTxList.Transactions {
			txType := tx["type"]

			// Let only Invoke transactions stay
			if txType != float64(16) {
				continue
			}

			txId := tx["id"]
			txSender := tx["sender"].(string)

			wrappedStateChanges := entities.FetchStateChanges(txId.(string))

			stateChanges := wrappedStateChanges.StateChanges

			if !(stateChanges.Data != nil && len(stateChanges.Data) > 0) {
				return
			}

			fmt.Printf("TX: %v L: %v; StateChange: %v \n", txId, len(stateChanges.Data), *stateChanges.Data[0])

			if txSender == "" {
				continue
			}
			if len(stateChanges.Data) != 12 {
				continue
			}

			for i, change := range stateChanges.Data {
				changeKey := *(*change).Key

				if changeKey == entities.OrderBookKey || changeKey == entities.OrderFirstKey {
					continue
				}

				if !strings.Contains(changeKey, "order") {
					break
				}

				splittedKey := strings.Split(changeKey, delimiter)

				if len(splittedKey) < 3 {
					continue
				}

				orderId := splittedKey[len(splittedKey) - 1]
				dict := entities.MapStateChangesDataToDict(stateChanges)
				fmt.Printf("Dict: %v \n", dict)
				entity := uc.ScDelegate.BondsOrder.MapItemToModel(orderId, dict)

				uc.DbDelegate.DbConnection.Update(entity)

				fmt.Printf("Entity: %+v \n", entity)
				fmt.Printf("Data key immutable part: %v \n", changeKey)
				fmt.Printf("TX ID: %v, Sender is: %v \n", txId, txSender)

				fmt.Printf("Data #%v: %+v \n", i + 1, *change)
				fmt.Printf("%v , %v , %v \n", *(*change).Key, (*change).Value, *(*change).Type)

				break
			}
		}
	}
}

func (uc *UpdateController) UpdateAllData () {
	uc.DbDelegate.HandleRecordsUpdate()
}

func (uc *UpdateController) StartConstantUpdating () {
	frequency := os.Getenv("UPDATE_FREQUENCY")
	duration, convErr := strconv.Atoi(frequency)

	if convErr != nil {
		fmt.Println(convErr)
		return
	}
	duration = int(time.Duration(duration) * time.Millisecond)

	for {
		uc.UpdateAllData()
		time.Sleep(time.Duration(duration))
	}
}