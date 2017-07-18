package main

import (
	"io/ioutil"
	"log"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IPScore struct {
	ip      string
	weight  int
	first   bool
	score   float64
	rttAvg  float64
	rttMdev float64
	lost    float64
}

func NewIPScore(ip string, weight int) *IPScore {
	return &IPScore{
		ip:     ip,
		weight: weight,
		first:  true,
	}
}

func (s *IPScore) lostScore(lost float64) float64 {
	ret := 100 / math.Pow((1+math.Pow(math.E, (-16.0*lost))), 2)
	return ret
}

func (s *IPScore) calculateScore(rttavg, rttmdev, lost float64) float64 {
	ret := rttavg*s.lostScore(lost-0.135) + 2*rttmdev
	return ret
}

func (s *IPScore) UpdateScore(rttAvg, rttMdev, lost float64) {
	if s.first {
		s.first = false
		s.score = s.calculateScore(rttAvg, rttMdev, lost)
		s.rttAvg = rttAvg
		s.rttMdev = rttMdev
		s.lost = lost
	} else {
		nrttAvg := (rttAvg + s.rttAvg) / 2
		nrttMdev := (rttMdev + s.rttMdev) / 2
		nlost := (lost + s.lost) / 2
		s.score = s.calculateScore(nrttAvg, nrttMdev, nlost)
		s.rttAvg = nrttAvg
		s.rttMdev = nrttMdev
		s.lost = nlost
	}
}

type BackendChecker struct {
	backends      []string
	backendScores map[string]*IPScore
	BestBackend   string
	running       bool
}

func NewBackendChecker(backends []string) *BackendChecker {
	backendScores := map[string]*IPScore{}
	firstBackend := ""
	for i, backend := range backends {
		addr, err := net.ResolveTCPAddr("tcp4", backend)
		if err == nil {
			if firstBackend == "" {
				firstBackend = backend
			}
			backendScores[backend] = NewIPScore(addr.IP.String(), (i+1)*10)
		}
	}
	return &BackendChecker{
		backends:      backends,
		backendScores: backendScores,
		BestBackend:   firstBackend,
		running:       true,
	}
}

func (c *BackendChecker) chooseBestBackend() {
	var bscore float64
	var bweight int
	first := true
	bback := ""
	for backend, score := range c.backendScores {
		if first {
			first = false
			bscore = score.score
			bweight = score.weight
			bback = backend
			continue
		}
		needSwap := false
		diff := bscore - score.score
		if diff > 80.0 {
			needSwap = true
		} else if math.Abs(diff) < 80.0 {
			if bweight > score.weight {
				needSwap = true
			}
		}
		if needSwap {
			bscore = score.score
			bweight = score.weight
			bback = backend
		}
	}
	if bback != "" {
		c.BestBackend = bback
	}
}

func (c *BackendChecker) Start() {
	go c.run()
}

func (c *BackendChecker) run() {
	var wg sync.WaitGroup
	for c.running {
		for _, score := range c.backendScores {
			wg.Add(1)
			go c.doPingAndUpdateScore(score, &wg)
		}
		wg.Wait()
		c.chooseBestBackend()
		c.reportBackendStatus()
		time.Sleep(50 * time.Second)
	}
}

func (c *BackendChecker) reportBackendStatus() {
	for _, score := range c.backendScores {
		log.Printf("%s, %d, %.1f, %.2f, %.2f, %.2f%%\n", score.ip, score.weight, score.score, score.rttAvg, score.rttMdev, score.lost*100.0)
	}
	log.Println("Best Backend:", c.BestBackend)
}

func (c *BackendChecker) Stop() {
	c.running = false
}

func (c *BackendChecker) doPingAndUpdateScore(score *IPScore, wg *sync.WaitGroup) {
	defer wg.Done()
	rttavg, rttmdev, lost, err := c.ping(score.ip)
	if err != nil {
		log.Println(err)
		return
	}
	score.UpdateScore(rttavg, rttmdev, lost)
}

func (c *BackendChecker) ping(ip string) (rttavg float64, rttmdev float64, lost float64, err error) {
	cmd := exec.Command("/sbin/ping", "-i", "0.1", "-c", "100", ip)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	err = cmd.Start()
	if err != nil {
		return
	}
	outBuf, err := ioutil.ReadAll(stdout)
	if err != nil {
		return
	}
	err = cmd.Wait()
	if err != nil {
		return
	}
	rttavg, rttmdev, lost = c.parsePingOutput(string(outBuf))
	return
}

func (c *BackendChecker) parsePingOutput(out string) (rttavg float64, rttmdev float64, lost float64) {
	var err error
	lostLine := ""
	rttLine := ""
	for _, rline := range strings.Split(out, "\n") {
		line := strings.TrimSpace(rline)
		if strings.Contains(line, "packet loss") {
			lostLine = line
			continue
		}
		if strings.Contains(line, "min/avg/max") {
			rttLine = line
			continue
		}
	}
	if rttLine == "" && lostLine != "" {
		// This mean all packet lost, so we should give a very big result
		rttavg = 1000.0
		rttmdev = 1000.0
		lost = 100.0
		return
	}
	// Parse rtt line
	rttParta := strings.Split(rttLine, "=")
	if len(rttParta) < 2 {
		return
	}
	rttPartb := strings.Split(strings.TrimSpace(rttParta[1]), " ")
	if len(rttPartb) == 0 {
		return
	}
	rttPartc := strings.Split(rttPartb[0], "/")
	if len(rttPartc) == 4 {
		rttavg, err = strconv.ParseFloat(rttPartc[1], 64)
		if err != nil {
			return
		}
		rttmdev, err = strconv.ParseFloat(rttPartc[3], 64)
		if err != nil {
			return
		}
	}
	// Parse lost line
	for _, lpart := range strings.Split(lostLine, ",") {
		if strings.Contains(lpart, "packet loss") {
			// We got lost part
			lostParta := strings.Split(strings.TrimSpace(lpart), " ")
			if len(lostParta) == 0 {
				return
			}
			lostParta0 := lostParta[0]
			if len(lostParta0) < 1 {
				return
			}
			lostData := lostParta0[0 : len(lostParta0)-1]
			losta, err := strconv.ParseFloat(lostData, 64)
			if err != nil {
				return
			}
			lost = losta / 100.0
		}
	}
	return
}
