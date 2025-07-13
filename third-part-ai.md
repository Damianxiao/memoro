package main

import (
   "fmt"
   "strings"
   "net/http"
   "io/ioutil"
)

func main() {

   url := "https://api.gpt.ge/v1/chat/completions"
   method := "POST"

   payload := strings.NewReader(`{
    "model": "gpt-4o",
    "messages": [
        {
            "role": "user",
            "content": "晚上好"
        }
    ],
    "max_tokens": 1688,
    "temperature": 0.5,
    "stream": false
}`)

   client := &http.Client {
   }
   req, err := http.NewRequest(method, url, payload)

   if err != nil {
      fmt.Println(err)
      return
   }
   req.Header.Add("Content-Type", "application/json")

   res, err := client.Do(req)
   if err != nil {
      fmt.Println(err)
      return
   }
   defer res.Body.Close()

   body, err := ioutil.ReadAll(res.Body)
   if err != nil {
      fmt.Println(err)
      return
   }
   fmt.Println(string(body))
}

response:

id
string 
必需
object
string 
必需
created
integer 
必需
model
string 
必需
choices
array [object {4}] 
必需
index
integer 
可选
message
object 
可选
logprobs
null 
可选
finish_reason
string 
可选
usage
object 
必需
prompt_tokens
integer 
必需
completion_tokens
integer 
必需
total_tokens
integer 
必需
system_fingerprint
string 
必需


{
    "id": "chatcmpl-A1iMgDLzZtUJ9QDpfqDLxKH0zfUnp",
    "object": "chat.completion",
    "created": 1724972230,
    "model": "gpt-4o-2024-05-13",
    "choices": [
        {
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "晚上好！有什么我可以帮你的吗？",
                "refusal": null
            },
            "logprobs": null,
            "finish_reason": "stop"
        }
    ],
    "usage": {
        "prompt_tokens": 9,
        "completion_tokens": 10,
        "total_tokens": 19
    },
    "system_fingerprint": "fp_157b3831f5"
}