package main

import (
	"log"
	"net/http"
	"sync"
)

// thread safe id provider. Produces an int64 id from a thread safe counter
type IDProvider struct {
	lock    *sync.Mutex
	counter int64
}

func (p *IDProvider) getNextID() int64 {
	p.lock.Lock()
	p.counter++
	newID := p.counter
	p.lock.Unlock()
	return newID
}

var idp = &IDProvider{lock: &sync.Mutex{}}

// Storage stores a Pixel. It updates it from an incoming channel, and distributes the change to its listeners.
type Storage struct {
	data     *Pixel
	clients  map[int64]chan *Pixel
	dataLock *sync.RWMutex
}

// register new listener. Return the the client id and the current Pixel
func (st Storage) Register(ch chan *Pixel) (int64, *Pixel) {
	id := idp.getNextID()
	st.clients[id] = ch
	log.Println("register new client", id)

	st.dataLock.RLock()
	data := st.data
	st.dataLock.RUnlock()
	return id, data
}

// remove a listener
func (st *Storage) Deregister(id int64) {
	close(st.clients[id])
	delete(st.clients, id)
	log.Println("client de-registered", id)
}

// get updates from the Hat, and distribute it to the listener
func (st *Storage) do(ch <-chan *Pixel) {
	for data := range ch {
		st.dataLock.Lock()
		st.data = data
		st.dataLock.Unlock()
		for _, client := range st.clients {
			client <- data
		}
	}
}

func NewStorage(ch <-chan *Pixel) *Storage {
	st := &Storage{clients: map[int64]chan *Pixel{}, dataLock: &sync.RWMutex{}}

	go st.do(ch)

	return st
}

func (st Storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, _ := upgrader.Upgrade(w, r, nil) // error ignored for sake of simplicity

	ch := make(chan *Pixel)

	id, px := st.Register(ch)
	defer st.Deregister(id)

	if err := conn.WriteJSON(px); err != nil {
		log.Println(err)
		return
	}

	for px = range ch {
		if err := conn.WriteJSON(px); err != nil {
			log.Println(err)
			return
		}

	}
}
