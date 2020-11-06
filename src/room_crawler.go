package src

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type (
	room struct {
		id     string
		title  string
		price  string
		layout string
		size   string
	}

	roomPair struct {
		new room
		old room
	}

	diff struct {
		newRooms         []room
		updatedRoomPairs []roomPair
		removedRooms     []room
	}
)

const (
	spreadsheetSheetRange = "sheet1!A2:E100"
)

var (
	roomsURL              = os.Getenv("ROOMS_URL")
	spreadsheetID         = os.Getenv("SPREADSHEET_ID")
	slackWebhookURL       = os.Getenv("SLACK_WEBHOOK_URL")
	googleCredentialsJSON = os.Getenv("GOOGLE_CREDENTIALS_JSON")
)

// Execute execute crawling
func Execute() error {
	rooms, err := fetchAllRooms(roomsURL)
	if err != nil {
		return fmt.Errorf("failed to fetch rooms: %s", err.Error())
	}

	pRooms, err := loadPreviousRooms()
	if err != nil {
		return fmt.Errorf("failed to load previous rooms: %s", err.Error())
	}

	d := detectDiff(rooms, pRooms)
	if d == nil {
		fmt.Println("no diff detected.")
		return nil
	}

	errTexts := []string{}
	if err := notifyDiff(*d); err != nil {
		errTexts = append(errTexts, fmt.Sprintf("failed to notify diff: %s", err.Error()))
	}
	if err := saveRooms(rooms); err != nil {
		errTexts = append(errTexts, fmt.Sprintf("failed to save rooms: %s", err.Error()))
	}

	if len(errTexts) == 0 {
		return nil
	}
	return errors.New(strings.Join(errTexts, "\n"))
}

// NotifyError Notify error
func NotifyError(err error) error {
	payload, err := json.Marshal(map[string]interface{}{"blocks": map[string]interface{}{
		"type": "section",
		"text": map[string]interface{}{
			"type": "mrkdwn",
			"text": fmt.Sprintf(err.Error()),
		},
	}})
	if err != nil {
		return err
	}
	return notifySlack(payload)
}

func fetchAllRooms(url string) ([]room, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return nil, err
	}

	rooms := []room{}
	doc.Find("li.list-group-item").Each(func(index int, s *goquery.Selection) {
		if room := newRoom(s); room != nil {
			rooms = append(rooms, *room)
		}
	})

	return rooms, nil
}

func newRoom(listGroupItemLI *goquery.Selection) *room {
	dataID, exists := listGroupItemLI.Attr("data-id")
	if !exists {
		return nil
	}

	titles := []string{}
	listGroupItemLI.Find("span.prop-title-link").Each(func(index int, s *goquery.Selection) {
		titles = append(titles, strings.Trim(s.Text(), "  	\n"))
	})
	price := ""
	if priceDiv := listGroupItemLI.Find("div.price"); priceDiv != nil && priceDiv.Length() > 0 {
		if priceChildren := priceDiv.First().Children(); priceChildren != nil && priceChildren.Length() > 0 {
			price = strings.Trim(priceChildren.First().Text(), "  	\n")
		}
	}
	layout := ""
	if layoutTitleSpan := listGroupItemLI.Find("span:contains(間取り)"); layoutTitleSpan != nil {
		layout = strings.Trim(layoutTitleSpan.Parent().Children().Last().Text(), "  	\n")
	}
	size := ""
	if sizeTitleSpan := listGroupItemLI.Find("span:contains(専有面積)"); sizeTitleSpan != nil {
		size = strings.Trim(sizeTitleSpan.Parent().Children().Last().Text(), "  	\n")
	}

	return &room{
		id:     dataID,
		title:  strings.Join(titles, " "),
		price:  price,
		layout: layout,
		size:   size,
	}
}

func loadPreviousRooms() ([]room, error) {
	srv, err := getSheetService()
	if err != nil {
		return nil, err
	}
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, spreadsheetSheetRange).Do()
	if err != nil {
		return nil, err
	}

	rooms := []room{}
	for i, row := range resp.Values {
		if len(row) != 5 {
			fmt.Printf("row %d is malformed: %v\n", i+2, row)
			continue
		}
		rooms = append(rooms, room{
			id:     row[0].(string),
			title:  row[1].(string),
			price:  row[2].(string),
			layout: row[3].(string),
			size:   row[4].(string)})
	}

	return rooms, nil
}

func detectDiff(rooms, previousRooms []room) *diff {
	d := &diff{}
	previousRoomsMap := make(map[string]room, len(previousRooms))
	for _, pRoom := range previousRooms {
		previousRoomsMap[pRoom.id] = pRoom
	}

	for _, room := range rooms {
		if pRoom, ok := previousRoomsMap[room.id]; ok {
			p := roomPair{new: room, old: pRoom}
			if p.hasDiff() {
				d.updatedRoomPairs = append(d.updatedRoomPairs, p)
			}
			delete(previousRoomsMap, room.id)
		} else {
			d.newRooms = append(d.newRooms, room)
		}
	}

	for _, pRoom := range previousRoomsMap {
		d.removedRooms = append(d.removedRooms, pRoom)
	}

	if reflect.DeepEqual(*d, diff{}) {
		return nil
	}

	return d
}

func notifyDiff(d diff) error {
	payload, err := newDiffSlackLayoutBlocks(d)
	fmt.Println(string(payload))
	if err != nil {
		return err
	}
	return notifySlack(payload)
}

func saveRooms(rooms []room) error {
	srv, err := getSheetService()
	if err != nil {
		return err
	}

	values := make([][]interface{}, 0, len(rooms))
	for _, room := range rooms {
		values = append(values, []interface{}{"'" + room.id, room.title, room.price, room.layout, room.size})
	}

	clearReq := sheets.BatchClearValuesRequest{Ranges: []string{spreadsheetSheetRange}}
	_, err = srv.Spreadsheets.Values.BatchClear(spreadsheetID, &clearReq).Do()
	if err != nil {
		return err
	}

	updateReq := &sheets.BatchUpdateValuesRequest{ValueInputOption: "USER_ENTERED"}
	updateReq.Data = append(updateReq.Data, &sheets.ValueRange{
		Range:  spreadsheetSheetRange,
		Values: values,
	})
	_, err = srv.Spreadsheets.Values.BatchUpdate(spreadsheetID, updateReq).Do()
	if err != nil {
		return err
	}

	return nil
}

func (r *room) url() string {
	return fmt.Sprintf("https://www.fudousan.or.jp/property/detail?p_no=%s", r.id)
}

func newDiffSlackLayoutBlocks(d diff) ([]byte, error) {
	blocks := []map[string]interface{}{}

	if len(d.newRooms) > 0 {
		for _, newRoom := range d.newRooms {
			newRoomBlock := map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf(
						"New: %s\n%s %s %s\n%s",
						newRoom.title,
						newRoom.price,
						newRoom.layout,
						newRoom.size,
						newRoom.url(),
					),
				},
			}
			blocks = append(blocks, newRoomBlock)
		}
	}

	if len(d.updatedRoomPairs) > 0 {
		for _, p := range d.updatedRoomPairs {
			updatedRoomBlock := map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf(
						"Updated: %s\n%s\n%s",
						p.new.title,
						p.new.price,
						p.new.url(),
					),
				},
			}
			blocks = append(blocks, updatedRoomBlock)
		}
	}

	if len(d.removedRooms) > 0 {
		for _, removedRoom := range d.removedRooms {
			removedRoomBlock := map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("Removed: %s", removedRoom.title),
				},
			}
			blocks = append(blocks, removedRoomBlock)
		}
	}

	return json.Marshal(map[string]interface{}{"blocks": blocks})
}

func (p *roomPair) hasDiff() bool {
	return !reflect.DeepEqual(p.new, p.old)
}

func notifySlack(payload []byte) error {
	resp, err := http.PostForm(
		slackWebhookURL,
		url.Values{"payload": {string(payload)}},
	)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getSheetService() (*sheets.Service, error) {
	return sheets.NewService(
		context.Background(),
		option.WithCredentialsJSON([]byte(googleCredentialsJSON)),
		option.WithScopes("https://www.googleapis.com/auth/spreadsheets"))
}
