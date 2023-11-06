package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

var endpoint string

func oauth() string {
	var token, host string

	//認証
	fmt.Println("トークンを設定してください。APIで全権限付与した物を扱うので扱いには十分お気を付けください。")
	token = readInput("Token: ")

	fmt.Println("サーバーのホストを設定してください。https://の記載は不要でドメインのみ記載してね。")
	host = readInput("Host: ")

	endpoint = "https://" + host + "/api/"

	return token
}

func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func post(api string, args map[string]interface{}) ([]byte, error) {
	requestBody, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	res, err := http.Post(endpoint+api, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error: %d", res.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}

type User struct {
	Name        string `json:"name"`
	Username    string `json:"username"`
	NotesCount  int    `json:"notesCount"`
	Id          string `json:"id"`
	PinnedNotes []Note `json:"pinnedNotes"`
}

type Note struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type Error struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Id      string `json:"id"`
	Kind    string `json:"kind"`
}

//ピンどめされてたら外してから抹消する処理

func UnpinNote(noteId, token string) error {
	args := map[string]interface{}{
		"noteId": noteId,
		"i":      token,
	}
	return Post("i/unpin", args, nil)
}

func main() {
	//ユーザー情報の照会
	token := oauth()
	me, err := FetchUser("i", map[string]interface{}{"i": token})
	if err != nil {
		fmt.Println("照会中にエラーが発生しました:", err)
		return
	}

	fmt.Println("ノート数を補足:")
	fmt.Printf(" %s @%s\n", me.Name, me.Username)
	fmt.Printf(" %d Notes\n", me.NotesCount)
	fmt.Printf(" id: %s\n", me.Id)

	//ピン留めの解除
	pinnedCount := 0

	for _, note := range me.PinnedNotes {
		err := UnpinNote(note.Id, token)
		if err != nil {
			fmt.Printf("Error unpinning note: %v\n", err)
		} else {
			fmt.Printf("Unpinned note: %v\n", note)
			pinnedCount++
		}
	}

	fmt.Printf("Unpinned %d notes\n", pinnedCount)

	notes := []Note{}
	needsFetchingAllNotes := true

	if _, err := os.Stat("notes.json"); err == nil {
		var yesno string
		fmt.Println("You have `notes.json`. By using this file, no longer need to make API requests for all notes. May I import this file?")
		for yesno != "y" && yesno != "n" {
			fmt.Print(" (Y,n) > ")
			fmt.Scanln(&yesno)
		}

		if yesno == "y" {
			needsFetchingAllNotes = false
			content, err := ioutil.ReadFile("notes.json")
			if err != nil {
				fmt.Println("Error reading file:", err)
			} else {
				var n []Note
				if err := json.Unmarshal(content, &n); err != nil {
					fmt.Println("Error decoding JSON:", err)
				} else {
					notes = append(notes, n...)
					for _, note := range n {
						fmt.Printf("Imported: %v\n", note)
					}
				}
			}
		}
	}

	if needsFetchingAllNotes {
		fmt.Println("Fetching your all notes. It takes some time...")
		untilId := ""

		for {
			fetched, err := GetUsersNotes(me.Id, untilId, token)
			if err != nil {
				fmt.Printf("Error fetching notes: %v\n", err)
				break
			}

			if len(fetched) == 0 {
				break
			}

			untilId = fetched[len(fetched)-1].Id
			notes = append(notes, fetched...)
			fmt.Printf("Fetched %d notes.\n", len(notes))
		}
	}

	fmt.Printf("Fetched your %d notes!\n", len(notes))

	//取得したnotesをcreatedAtの昇順にソートする
	notes = orderByCreatedAt(notes)

	for i := 0; i < len(notes); i++ {
		note := notes[i]
		err := DeleteNote(note.Id, token)
		if err != nil {
			fmt.Printf("Error deleting note %d/%d: %v\n", i+1, len(notes), err)
			fmt.Println("Retry after 15 minutes")
			time.Sleep(15 * time.Minute)
			i--
		} else {
			fmt.Printf("Deleted note %d/%d\n", i+1, len(notes))
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Println("Press ENTER to exit")
	fmt.Scanln()
}

func orderByCreatedAt(notes []Note) []Note {

	return notes
}

func GetUsersNotes(userId, untilId, token string) ([]Note, error) {
	args := map[string]interface{}{}
	if untilId == "" {
		args = map[string]interface{}{
			"userId": userId,
			"limit":  100,
			"i":      token,
		}
	} else {
		args = map[string]interface{}{
			"userId":  userId,
			"untilId": untilId,
			"limit":   100,
			"i":       token,
		}
	}

	notes, err := FetchNotes("users/notes", args)
	return notes, err
}

func FetchUser(api string, args map[string]interface{}) (User, error) {
	user := User{}
	err := Post(api, args, &user)
	return user, err
}

func FetchNotes(api string, args map[string]interface{}) ([]Note, error) {
	notes := []Note{}
	err := Post(api, args, &notes)
	return notes, err
}

func Post(api string, args map[string]interface{}, result interface{}) error {
	requestBody, err := json.Marshal(args)
	if err != nil {
		return err
	}

	res, err := http.Post(endpoint+api, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: %d", res.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return err
		}
	}

	return nil
}

func DeleteNote(noteId, token string) error {
	args := map[string]interface{}{
		"noteId": noteId,
		"i":      token,
	}

	return Post("notes/delete", args, nil)
}
