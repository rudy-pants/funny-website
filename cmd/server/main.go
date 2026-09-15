package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddr = "127.0.0.1:8080"

type joke struct {
	Setup     string `json:"setup"`
	Punchline string `json:"punchline"`
}

type region struct {
	ID        string
	Name      string
	Category  string
	Blurb     string
	Path      string
	LabelX    int
	LabelY    int
	Jokes     []joke
	UnlockIDs []string
}

type jokeView struct {
	Region region
	Joke   joke
	Pick   int
}

type discoveryView struct {
	Region  region
	Joke    jokeView
	Unlocks []region
}

type pageData struct {
	Title            string
	Regions          []region
	RegionCount      int
	InitialDiscovery *discoveryView
}

type application struct {
	template *template.Template
	regions  map[string]region
	order    []region
}

func main() {
	page, err := template.New("index.html").Funcs(template.FuncMap{
		"add": func(left, right int) int { return left + right },
	}).ParseFiles("templates/index.html")
	if err != nil {
		log.Fatalf("load page template: %v", err)
	}

	app := newApplication(page)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("joke atlas listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newApplication(page *template.Template) *application {
	regions := []region{
		{
			ID:       "dunes",
			Name:     "Dad Joke Dunes",
			Category: "Classic dad jokes",
			Blurb:    "Warm, wholesome, and gloriously predictable.",
			Path:     "M107 181C121 145 177 126 235 142C278 153 302 187 280 222C258 256 211 244 166 257C122 269 85 232 107 181Z",
			LabelX:   192,
			LabelY:   194,
			Jokes: []joke{
				{Setup: "I only know 25 letters of the alphabet.", Punchline: "I don't know y."},
				{Setup: "What do you call a fake noodle?", Punchline: "An impasta."},
				{Setup: "I used to hate facial hair...", Punchline: "But then it grew on me."},
			},
			UnlockIDs: []string{"prairie"},
		},
		{
			ID:       "prairie",
			Name:     "Pun Prairie",
			Category: "Big-field wordplay",
			Blurb:    "Where every horizon has a punchline hiding behind it.",
			Path:     "M352 120C387 94 451 106 496 128C540 149 579 142 609 171C637 199 620 237 579 242C535 248 503 227 462 243C418 260 365 239 357 204C351 177 330 145 352 120Z",
			LabelX:   487,
			LabelY:   177,
			Jokes: []joke{
				{Setup: "I'm reading a book about anti-gravity.", Punchline: "It's impossible to put down."},
				{Setup: "Did you hear about the restaurant on the moon?", Punchline: "Great food, but no atmosphere."},
				{Setup: "I wanted to be a calendar model...", Punchline: "But my days were numbered."},
			},
			UnlockIDs: []string{"keys", "lagoon"},
		},
		{
			ID:       "keys",
			Name:     "Knock-Knock Keys",
			Category: "Doorway comedy",
			Blurb:    "A chain of tiny islands with excellent timing.",
			Path:     "M723 113C755 91 811 105 836 131C859 155 850 185 818 192C789 199 777 226 741 216C709 207 690 177 700 148C705 132 710 122 723 113Z",
			LabelX:   777,
			LabelY:   157,
			Jokes: []joke{
				{Setup: "Knock, knock.\nWho's there? Lettuce.", Punchline: "Lettuce in—it's chilly out here!"},
				{Setup: "Knock, knock. Who's there? Cow says.", Punchline: "Cow says who? No, a cow says moo."},
				{Setup: "Knock, knock. Who's there? Tank.", Punchline: "Tank who? You're welcome!"},
			},
			UnlockIDs: []string{"borough"},
		},
		{
			ID:       "lagoon",
			Name:     "One-Liner Lagoon",
			Category: "Low-tide one-liners",
			Blurb:    "Short jokes, long ripples. Please mind the splash zone.",
			Path:     "M126 348C151 319 194 321 231 338C267 355 301 342 331 365C361 388 352 429 319 447C284 465 252 448 215 460C176 472 132 454 117 420C107 395 109 368 126 348Z",
			LabelX:   236,
			LabelY:   395,
			Jokes: []joke{
				{Setup: "I told my suitcase there would be no vacations this year.", Punchline: "Now I'm dealing with emotional baggage."},
				{Setup: "I asked the librarian if the library had books on paranoia.", Punchline: "She whispered, ‘They're right behind you.’"},
				{Setup: "I'm friends with all the electricians.", Punchline: "We have good current connections."},
			},
			UnlockIDs: []string{"gulch"},
		},
		{
			ID:       "gulch",
			Name:     "Groan Gulch",
			Category: "Dry humor canyon",
			Blurb:    "The air is dry, the jokes are drier, and the tumbleweeds are laughing.",
			Path:     "M404 346C431 314 481 314 518 337C551 358 588 349 616 379C643 408 627 445 591 458C551 473 515 451 479 464C440 479 397 454 389 416C383 389 386 367 404 346Z",
			LabelX:   506,
			LabelY:   397,
			Jokes: []joke{
				{Setup: "I tried to catch fog yesterday.", Punchline: "Mist."},
				{Setup: "I got a job at a bakery...", Punchline: "Because I kneaded dough."},
				{Setup: "I wondered why the baseball kept getting bigger.", Punchline: "Then it hit me."},
			},
			UnlockIDs: []string{"borough"},
		},
		{
			ID:       "borough",
			Name:     "Button Borough",
			Category: "Tiny tech chuckles",
			Blurb:    "A little digital district with a surprisingly analog sense of humor.",
			Path:     "M701 318C727 287 777 291 810 313C842 334 884 325 902 358C919 389 898 421 864 429C831 437 804 419 769 435C731 452 690 426 684 388C680 361 686 336 701 318Z",
			LabelX:   794,
			LabelY:   369,
			Jokes: []joke{
				{Setup: "Why did the computer go to the doctor?", Punchline: "It had a virus."},
				{Setup: "What do you call a sleeping bull?", Punchline: "A bulldozer."},
				{Setup: "My pencil broke...", Punchline: "So it was pointless."},
			},
			UnlockIDs: []string{"dunes"},
		},
	}

	byID := make(map[string]region, len(regions))
	for _, item := range regions {
		byID[item.ID] = item
	}
	return &application{template: page, regions: byID, order: regions}
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/jokes", a.handleJokes)
	mux.HandleFunc("/unlock", a.handleUnlock)
	mux.HandleFunc("/api/joke", a.handleAPIJoke)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return withSecurityHeaders(mux)
}

func (a *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !getRequest(w, r) {
		return
	}
	a.renderPage(w, pageData{
		Title:       "Joke Atlas",
		Regions:     a.order,
		RegionCount: len(a.order),
	})
}

func (a *application) handleJokes(w http.ResponseWriter, r *http.Request) {
	if !getRequest(w, r) {
		return
	}
	view, err := a.jokeView(r)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	if isHTMX(r) {
		renderTemplate(w, a.template, "joke", view)
		return
	}
	discovery := a.discovery(view)
	a.renderPage(w, pageData{
		Title:            view.Region.Name + " · Joke Atlas",
		Regions:          a.order,
		RegionCount:      len(a.order),
		InitialDiscovery: &discovery,
	})
}

func (a *application) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if !getRequest(w, r) {
		return
	}
	view, err := a.jokeView(r)
	if err != nil {
		writeRequestError(w, err)
		return
	}
	discovery := a.discovery(view)
	if isHTMX(r) {
		renderTemplate(w, a.template, "unlock", discovery)
		return
	}
	a.renderPage(w, pageData{
		Title:            discovery.Region.Name + " · Joke Atlas",
		Regions:          a.order,
		RegionCount:      len(a.order),
		InitialDiscovery: &discovery,
	})
}

func (a *application) handleAPIJoke(w http.ResponseWriter, r *http.Request) {
	if !getRequest(w, r) {
		return
	}
	view, err := a.jokeView(r)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(struct {
		Region struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"region"`
		Joke joke `json:"joke"`
		Pick int  `json:"pick"`
	}{
		Region: struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
		}{ID: view.Region.ID, Name: view.Region.Name, Category: view.Region.Category},
		Joke: view.Joke,
		Pick: view.Pick,
	})
}

func (a *application) jokeView(r *http.Request) (jokeView, error) {
	id := strings.TrimSpace(r.URL.Query().Get("region"))
	item, ok := a.regions[id]
	if !ok {
		return jokeView{}, requestError{status: http.StatusBadRequest, message: "choose a valid map region"}
	}

	pick, err := choosePick(r.URL.Query().Get("pick"), len(item.Jokes))
	if err != nil {
		return jokeView{}, err
	}
	return jokeView{Region: item, Joke: item.Jokes[pick], Pick: pick}, nil
}

func (a *application) discovery(view jokeView) discoveryView {
	item := discoveryView{Region: view.Region, Joke: view}
	for _, id := range view.Region.UnlockIDs {
		if unlocked, ok := a.regions[id]; ok {
			item.Unlocks = append(item.Unlocks, unlocked)
		}
	}
	return item
}

func choosePick(raw string, count int) (int, error) {
	if count == 0 {
		return 0, requestError{status: http.StatusInternalServerError, message: "this region has no jokes yet"}
	}
	if raw == "" {
		// The server keeps no per-visitor counter; time simply adds variety to
		// repeated stateless requests while an explicit pick remains testable.
		return int(time.Now().UnixNano() % int64(count)), nil
	}
	pick, err := strconv.Atoi(raw)
	if err != nil || pick < 0 || pick >= count {
		return 0, requestError{status: http.StatusBadRequest, message: "pick must be a valid joke number"}
	}
	return pick, nil
}

func getRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (a *application) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.template.ExecuteTemplate(w, "index", data); err != nil {
		log.Printf("render page: %v", err)
	}
}

func renderTemplate(w http.ResponseWriter, page *template.Template, name string, data any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

type requestError struct {
	status  int
	message string
}

func (e requestError) Error() string { return e.message }

func writeRequestError(w http.ResponseWriter, err error) {
	var reqErr requestError
	if errors.As(err, &reqErr) {
		writeError(w, reqErr.status, reqErr.message)
		return
	}
	writeError(w, http.StatusBadRequest, "could not read that request")
}

func writeJSONError(w http.ResponseWriter, err error) {
	var reqErr requestError
	status := http.StatusBadRequest
	message := "could not read that request"
	if errors.As(err, &reqErr) {
		status = reqErr.status
		message = reqErr.message
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, message, status)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
