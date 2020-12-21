//
// lurkcoin admin pages
// Copyright © 2020 by luk3yx
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//

package api

import (
	"crypto/sha512"
	"encoding/hex"
	"github.com/julienschmidt/httprouter"
	"html"
	"html/template"
	"io"
	"log"
	"lurkcoin"
	"net/http"
	"regexp"
	"strings"
)

const adminPagesHeader = `<!DOCTYPE html>
<html>
<head>
	<title>lurkcoin admin pages</title>
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/skeleton/2.0.4/skeleton.min.css" />
	<link rel="stylesheet" href="https://www.xeroxirc.net/style.css" />
	<meta name="viewport" content="width=device-width" />
</head>
<body>
<main style="padding: 1.5em;">`

const adminPagesFooter = `</main></body></html>`

const popOutCode = `
btn.style.display = "inline";
btn.addEventListener("click", () => {
	form.style.display = "block";
	form.style.transition = "ease-in-out 250ms transform";
	window.setTimeout(() => {
		form.style.transform = "scaleY(1)";
		form.style.maxHeight = form.scrollHeight.toString() + "px";
		btn.style.opacity = "0.5";
		btn.style.pointerEvents = "none";
		window.location.hash = "#" + form.id;
	}, 25);
	btn.blur();
});
function hideForm() {
	form.style.transition = "ease-in-out 250ms";
	form.style.transform = "scaleY(0)";
	form.style.maxHeight = "0";
	btn.style.opacity = "1";
	btn.style.pointerEvents = "";
}
`

const serverListTemplate = adminPagesHeader + `
<h2>Server list</h2>
<i>Total: {{len .Summaries}} server(s).</i>
<table>
	<thead>
		<tr>
			<th>Name</th>
			<th>Balance</th>
			<th>Target balance</th>
			<th>Pending transactions</th>
			<th>...</th>
		</tr>
	</thead>
	<tbody>
		{{range $summary := .Summaries}}
			<tr>
				<td>{{$summary.Name}}</td>
				<td>{{$summary.Balance}}</td>
				<td>{{$summary.TargetBalance}}</td>
				<td>{{$summary.PendingTransactionCount}}</td>
				<td><a href="/admin/edit/{{$summary.UID}}">Edit</a></td>
			</tr>
		{{end}}
	</tbody>
</table>

{{if .AllowEditing}}
	<noscript>
		<h4>JavaScript is required to edit database entries.</h4>
	</noscript>

	<button id="new-server" class="button-primary">New server</button>
	{{if .AllowDatabaseDownload}}
		<a href="/admin/backup" class="button">Download database backup</a>
	{{end}}

	<style>
		html {
			scroll-behavior: smooth;
		}
		#new-server {
			display: none;
			transition: ease-in-out 250ms;
		}
		#create-server {
			padding-top: 2em;
			display: none;
			transform: scaleY(0);
			transform-origin: top center;
			max-height: 0;
		}
		#username-field {
			width: 100%;
		}
	</style>

	<form autocomplete="off" method="post" action="/admin/create-server"
			id="create-server">
		<h3>Create new server</h3>
		<input type="hidden" name="csrfToken" value={{.CSRFToken}} />
		<div style="display: inline-block;">
			Username<br/>
			<input type="text" name="username" minlength="3" maxlength="32"
				required="required" id="username-field" /><br/>
			<input type="submit" name="submit" class="button-primary"
				value="Create" />
			<button type="button" onclick="hideForm()">Cancel</button>
		</div>
	</form>

	<script>
		"use strict";
		const btn = document.getElementById("new-server");
		const form = document.getElementById("create-server");
		` + popOutCode + `
	</script>
{{else}}
	<i>You may not edit the database.</i>
{{end}}
` + adminPagesFooter

const currencyInput = `type="text" pattern="¤?[0-9,_]+(\.[0-9,_]+)?"`
const infoTemplate = adminPagesHeader + `
<style>
#form-inner input, #form-inner label, #message {
	transition: ease-in-out 200ms;
	text-overflow: ellipsis;
}
#form-inner input[disabled="disabled"] {
	border-color: transparent;
	background: none;
	color: inherit;
}
{{if .AllowEditing}}
	html {
		scroll-behavior: smooth;
	}
	#edit-btn, #edit-btn ~ .button, #edit-btn ~ button {
		display: none;
		transition: ease-in-out 250ms;
	}
	#delete-server {
		display: none;
		transform: scaleY(0);
		transform-origin: top center;
		max-height: 0;
	}
	#regenerate-token {
		margin-bottom: 2rem;
	}
	#regenerate-token + label {
		display: inline-block;
		vertical-align: middle;
		user-select: none;
	}
	#regenerate-token[disabled="disabled"],
			#regenerate-token[disabled="disabled"] + label {
		opacity: 0.5;
	}
{{end}}
</style>

<a href="/admin">Go back</a>
<h3>Server: {{.Server.Name}}</h3>
{{if .Message}}
	<h5 id="message" style="white-space: pre-line;">{{.Message}}</h5>
{{end}}
<h4>Basic information</h4>
<form autocomplete="off" method="post" action="{{.Server.UID}}">
	{{if .AllowEditing}}
		<input type="hidden" name="csrfToken" value="{{.CSRFToken}}" />
		<input type="hidden" name="oldBalance"
			value="{{.Server.GetBalance.RawString}}" />
		<input type="hidden" name="oldTargetBalance"
			value="{{.Server.GetTargetBalance.RawString}}" />
		<input type="hidden" name="oldWebhookURL"
			value="{{.Server.WebhookURL}}" />
	{{end}}
	<p id="form-inner">
		Balance<br/>
		<input ` + currencyInput + ` name="balance"
			value="{{.Server.GetBalance}}" disabled="disabled" />
		<br/>
		Target balance
		<br/>
		<input ` + currencyInput + ` name="targetBalance"
			value="{{.Server.GetTargetBalance}}" disabled="disabled" />
		<br/>
		Webhook URL<br/>
		<input type="url" value="{{.Server.WebhookURL}}" placeholder="(none)"
		 	disabled="disabled" name="webhookURL" />

		{{if .AllowEditing}}
			<br/>
			<input type="checkbox" id="regenerate-token"
				disabled="disabled" name="regenerateToken" />
			<label for="regenerate-token">
				Regenerate token
			</label>
			<br/>
			<button type="button" id="edit-btn"
				class="button-primary">Edit</button>
			<input type="submit" value="Save" class="button button-primary"
				disabled="disabled" />
			<button type="button" id="delete-btn">Delete</button>
			<a href="{{.Server.UID}}" class="button">Cancel</a>
			<script>
				document.getElementById("edit-btn").style.display = "inline";
				document.getElementById("delete-btn").style.display = "inline";
			</script>
		{{end}}
	</p>
</form>

<h4>History</h4>
<table>
	<thead>
		<tr>
			<th>ID</th>
			<th>Source</th>
			<th>Source server</th>
			<th>Target</th>
			<th>Target server</th>
			<th>Sent amount</th>
			<th>Amount</th>
			<th>Received amount</th>
			<th>Time</th>
			<th>Revertable</th>
		</tr>
	</thead>
	<tbody>
		{{range $transaction := .Server.GetHistory}}
			<tr>
				<td>{{$transaction.ID}}</td>
				<td>{{$transaction.Source}}</td>
				<td>{{$transaction.SourceServer}}</td>
				<td>{{$transaction.Target}}</td>
				<td>{{$transaction.TargetServer}}</td>
				<td>{{$transaction.SentAmount.RawString}}</td>
				<td>{{$transaction.Amount}}</td>
				<td>{{$transaction.ReceivedAmount.RawString}}</td>
				<td>{{$transaction.GetTime}}</td>
				<td>{{$transaction.Revertable | YesNo}}</td>
			</tr>
		{{end}}
	</tbody>
</table>

{{if .AllowEditing}}
	<form autocomplete="off" method="post" action="/admin/delete"
			id="delete-server">
		<h3>Delete server</h3>
		<b>This action cannot be undone.</b><br/>
		To confirm the server deletion, please type the server's name
		(<code>{{.Server.Name}}</code>) below.<br/><br/>
		<input type="hidden" name="csrfToken" value={{.CSRFToken}} />
		<input type="hidden" name="server-uid" value={{.Server.UID}} />
		<input type="text" name="delete-uid" /><br/>
		<input type="submit" name="delete" class="button-primary"
			value="Delete server" />
		<button type="button" onclick="hideForm()">Cancel</button>
	</form>

	<script>
		"use strict";
		const p = document.getElementById("form-inner");
		const editBtn = document.getElementById("edit-btn");
		const btn = document.getElementById("delete-btn");
		editBtn.addEventListener("click", () => {
			const msg = document.getElementById("message");
			if (msg) {
				msg.style.fontSize = "0";
				msg.style.margin = "0";
				msg.style.padding = "0";
			}
			p.removeChild(editBtn);
			p.removeChild(btn);
			for (let elem of p.children) {
				if (elem.tagName.toLowerCase() === "input")
					elem.removeAttribute("disabled");
			}
		});
		window.history.replaceState(null, null, "/admin/edit/{{.Server.UID}}");

		const form = document.getElementById("delete-server");
		` + popOutCode + `
	</script>
{{end}}
` + adminPagesFooter

type adminPagesSummary struct {
	UID                     string
	Name                    string
	Balance                 lurkcoin.Currency
	TargetBalance           lurkcoin.Currency
	PendingTransactionCount int
}

func parseNumbers(n1, n2 string) (lurkcoin.Currency, lurkcoin.Currency, bool) {
	n1 = strings.Replace(n1, ",", "", -1)
	n2 = strings.Replace(n2, ",", "", -1)

	var res1, res2 lurkcoin.Currency
	var err error
	res1, err = lurkcoin.ParseCurrency(n1)
	if err != nil {
		return c0, c0, false
	}
	res2, err = lurkcoin.ParseCurrency(n2)
	if err != nil {
		return c0, c0, false
	}
	return res1, res2, true
}

type AdminLoginDetails map[string]struct {
	PasswordHash          string `yaml:"password_hash"`
	HashAlgorithm         string `yaml:"hash_algorithm"`
	PasswordSalt          string `yaml:"password_salt"`
	AllowEditing          bool   `yaml:"allow_editing"`
	AllowDatabaseDownload bool   `yaml:"allow_database_download"`
}

// TODO: Provide a more secure hashing function.
func (self AdminLoginDetails) Validate(username, password string) bool {
	account, exists := self[username]
	if !exists {
		return false
	}

	password += account.PasswordSalt
	switch account.HashAlgorithm {
	case "sha512", "":
		rawHash := sha512.Sum512([]byte(password))
		return lurkcoin.ConstantTimeCompare(
			hex.EncodeToString(rawHash[:]),
			account.PasswordHash,
		)
	default:
		return false
	}
}

type csrfTokenManager map[string]string

// Generate one CSRF token per user
// TODO: Expiry
func (self csrfTokenManager) Get(username string) string {
	token, ok := self[username]
	if !ok {
		token = lurkcoin.GenerateToken()
		self[username] = token
	}
	return token
}

func writeAdminErrorPage(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(500)
	io.WriteString(w, adminPagesHeader+
		`<h2>An error has occurred!</h2>`+
		`<h5>`+html.EscapeString(msg)+`</h5>`+
		`<i>You can hurry back to the previous page, or learn to like`+
		` this error and then eventually grow old and die.</i>`+
		`<br/><br/>`+
		`<a class="button button-primary" href="/admin">Go back</a>`+
		adminPagesFooter)
}

func addAdminPages(router *httprouter.Router, db lurkcoin.Database,
	loginDetails AdminLoginDetails) {
	// TODO: Regenerate this often
	csrfTokens := make(csrfTokenManager)

	re, _ := regexp.Compile(`\s+`)
	var summaryTmpl, infoTmpl *template.Template
	var err error
	summaryTmpl, err = template.New("summary").Parse(
		re.ReplaceAllLiteralString(serverListTemplate, " "),
	)
	if err != nil {
		panic(err)
	}
	infoTmpl, err = template.New("info").Funcs(template.FuncMap{
		"YesNo": func(boolean bool) string {
			if boolean {
				return "Yes"
			} else {
				return "No"
			}
		},
	}).Parse(re.ReplaceAllLiteralString(infoTemplate, " "))
	if err != nil {
		panic(err)
	}

	accessDeniedPage := re.ReplaceAllLiteralString(
		adminPagesHeader+
			`<h1>`+
			`Sorry, you do not have access to this resource at `+
			`this time.`+
			`</h1>`+
			adminPagesFooter,
		" ",
	)
	authenticate := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		w.Header().Set("Cache-Control", "no-store")
		username, password, ok := r.BasicAuth()
		if ok && loginDetails.Validate(username, password) {
			return username, true
		}
		w.Header().Set(
			"WWW-Authenticate",
			`Basic realm="lurkcoin admin pages", charset="UTF-8"`,
		)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(401)
		io.WriteString(w, accessDeniedPage)
		return "", false
	}
	authenticateWithCSRF := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		username, ok := authenticate(w, r)
		if !ok {
			return username, ok
		}
		if !loginDetails[username].AllowEditing {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(401)
			io.WriteString(w, accessDeniedPage)
			return username, false
		}
		r.ParseForm()
		t, ok := csrfTokens[username]
		if !ok || !lurkcoin.ConstantTimeCompare(r.Form.Get("csrfToken"), t) {
			w.WriteHeader(500)
			io.WriteString(w, "Please try again.")
			return username, false
		}
		return username, true
	}

	router.GET("/admin", func(w http.ResponseWriter, r *http.Request,
		_ httprouter.Params) {
		username, ok := authenticate(w, r)
		if !ok {
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		var summaries []*adminPagesSummary
		var totalPendingTransactions int

		lurkcoin.ForEach(db, func(server *lurkcoin.Server) error {
			pendingTransactionCount := len(server.GetPendingTransactions())
			totalPendingTransactions += pendingTransactionCount
			summaries = append(summaries, &adminPagesSummary{
				server.UID,
				server.Name,
				server.GetBalance(),
				server.GetTargetBalance(),
				pendingTransactionCount,
			})
			return nil
		}, false)

		var data struct {
			Summaries             []*adminPagesSummary
			AllowEditing          bool
			AllowDatabaseDownload bool
			CSRFToken             string
		}
		data.Summaries = summaries
		d := loginDetails[username]
		data.AllowEditing = d.AllowEditing
		data.AllowDatabaseDownload = d.AllowDatabaseDownload
		if d.AllowEditing {
			data.CSRFToken = csrfTokens.Get(username)
		}

		err := summaryTmpl.Execute(w, data)
		if err != nil {
			panic(err)
		}
	})

	serverInfo := func(w http.ResponseWriter, r *http.Request,
		serverName, username, msg string) {
		servers, ok, _ := db.GetServers([]string{serverName})
		if !ok {
			w.WriteHeader(404)
			return
		}
		server := servers[0]
		defer db.FreeServers([]*lurkcoin.Server{server}, false)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		var data struct {
			Server       *lurkcoin.Server
			CSRFToken    string
			Message      string
			AllowEditing bool
		}
		data.Server = server
		data.CSRFToken = csrfTokens.Get(username)
		data.Message = msg
		data.AllowEditing = loginDetails[username].AllowEditing
		err := infoTmpl.Execute(w, data)
		if err != nil {
			panic(err)
		}
	}

	router.GET("/admin/edit/:server", func(w http.ResponseWriter,
		r *http.Request, params httprouter.Params) {
		username, ok := authenticate(w, r)
		if !ok {
			return
		}
		serverInfo(w, r, params.ByName("server"), username, "")
	})

	router.POST("/admin/edit/:server", func(w http.ResponseWriter,
		r *http.Request, params httprouter.Params) {
		adminUser, authenticated := authenticateWithCSRF(w, r)
		if !authenticated {
			return
		}

		// Get the server
		tr := lurkcoin.BeginDbTransaction(db)
		defer tr.Abort()
		servers, ok, _ := tr.GetServers(params.ByName("server"))
		if !ok {
			w.WriteHeader(404)
			return
		}
		server := servers[0]

		var msgs []string

		// Update the balance
		// This preserves any transactions after the initial page load.
		balance, oldBalance, ok := parseNumbers(
			r.Form.Get("balance"),
			r.Form.Get("oldBalance"),
		)
		if !ok {
			msgs = append(msgs, "Invalid balance specified!")
		} else if !balance.Eq(oldBalance) {
			if !server.ChangeBal(balance.Sub(oldBalance)) {
				server.ChangeBal(server.GetBalance())
			}
			msgs = append(msgs, "Balance updated!")
			log.Printf(
				"[Admin] User %#v changes balance of server %#v to %s",
				adminUser,
				server.Name,
				server.GetBalance(),
			)
		}

		// Update the target balance
		targetBalance, oldTargetBalance, ok := parseNumbers(
			r.Form.Get("targetBalance"),
			r.Form.Get("oldTargetBalance"),
		)
		if !ok {
			msgs = append(msgs, "Invalid target balance specified!")
		} else if !targetBalance.Eq(oldTargetBalance) {
			server.SetTargetBalance(targetBalance)
			msgs = append(msgs, "Target balance updated!")
			log.Printf(
				"[Admin] User %#v changes target balance of server %#v to %s",
				adminUser,
				server.Name,
				targetBalance,
			)
		}

		// Update the webhook URL
		webhookURL := r.Form.Get("webhookURL")
		if webhookURL != r.Form.Get("oldWebhookURL") {
			ok := server.SetWebhookURL(webhookURL)
			if ok {
				msgs = append(msgs, "Webhook URL updated!")
			} else {
				msgs = append(msgs, "Invalid webhook URL!")
			}
			log.Printf(
				"[Admin] User %#v changes webhook URL of server %#v to %#v",
				adminUser,
				server.Name,
				server.WebhookURL,
			)
		}

		if r.Form.Get("regenerateToken") == "on" {
			if len(msgs) == 0 {
				msgs = append(msgs, "New token: "+server.RegenerateToken())
				log.Printf(
					"[Admin] User %#v regenerates the token of server %#v",
					adminUser,
					server.Name,
				)
			} else {
				msgs = append(msgs, "Refusing to regenerate token as other"+
					" settings were changed.")
			}
		}

		// Finish the transaction
		uid := server.UID
		tr.Finish()

		serverInfo(w, r, uid, adminUser, strings.Join(msgs, "\n"))
	})

	router.POST("/admin/delete", func(w http.ResponseWriter,
		r *http.Request, params httprouter.Params) {
		adminUser, authenticated := authenticateWithCSRF(w, r)
		if !authenticated {
			return
		}

		serverUID := r.Form.Get("server-uid")
		if lurkcoin.HomogeniseUsername(r.Form.Get("delete-uid")) != serverUID {
			writeAdminErrorPage(w, "You didn't type the correct server UID!")
			return
		}

		if db.DeleteServer(serverUID) {
			log.Printf(
				"[Admin] User %#v deleted server %#v",
				adminUser,
				serverUID,
			)
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			writeAdminErrorPage(w, "Could not delete "+serverUID+"!")
		}
	})

	router.POST("/admin/create-server", func(w http.ResponseWriter,
		r *http.Request, params httprouter.Params) {
		adminUser, authenticated := authenticateWithCSRF(w, r)
		if !authenticated {
			return
		}
		serverName := strings.TrimSpace(r.Form.Get("username"))
		var msg string
		if len(serverName) < 3 || len(serverName) > 32 {
			msg = "The server name must be between 3 and 32 characters."
		} else {
			tr := lurkcoin.BeginDbTransaction(db)
			defer tr.Abort()
			server, ok := tr.CreateServer(serverName)
			if ok {
				log.Printf(
					"[Admin] User %#v created server %#v",
					adminUser,
					server.Name,
				)
				msg = "Token: " + server.Encode().Token
				tr.Finish()
				serverInfo(w, r, serverName, adminUser, msg)
				return
			}
			msg = "The specified server already exists!"
		}

		writeAdminErrorPage(w, msg)
	})

	router.GET("/admin/backup", func(w http.ResponseWriter,
		r *http.Request, params httprouter.Params) {
		username, ok := authenticate(w, r)
		if !ok {
			return
		}
		d := loginDetails[username]
		if !d.AllowEditing || !d.AllowDatabaseDownload {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(401)
			io.WriteString(w, accessDeniedPage)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set(
			"Content-Disposition",
			`attachment; filename="lurkcoin backup.json"`,
		)
		w.WriteHeader(http.StatusOK)
		err := lurkcoin.BackupDatabase(db, w)
		if err != nil {
			panic(err)
		}
	})
}
