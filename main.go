package main

import (
	"flag"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type statistics struct {
	sentPackets 	int
	receivedPackets int
	minRTT 			time.Duration
	maxRTT 			time.Duration
	avgRTT 			time.Duration
	totalTime 		time.Duration
}

func main() {

	//Set and parse the necessary flags
	IPaddress := flag.String("address", "", "IP or Hostname of the destination")
	ipVersion6 := flag.Bool("ipv6", false, "Bool true for using ipv6 address")
	flag.Parse()

	var connection4, connection6 *icmp.PacketConn
	var destinationAddress net.Addr
	var err error

	if *ipVersion6 {
		destinationAddress, err = net.ResolveIPAddr("ip6", *IPaddress)
		if err != nil {
			panic(err)
		}
		connection6, err = icmp.ListenPacket("ip6:ipv6-icmp", "::")
		if err != nil {
			panic(err)
		}
	} else {
		destinationAddress, err = net.ResolveIPAddr("ip", *IPaddress)
		if err != nil {
			panic(err)
		}
		connection4, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("PING %s (%s)\n", *IPaddress, destinationAddress)
	begin := time.Now()

	var stats statistics

	stats.minRTT = 5 * time.Second
	stats.maxRTT = 0
	stats.totalTime = 0

	stats.sentPackets = 0
	stats.receivedPackets = 0

	sigs := make(chan os.Signal, 1)
	var done bool

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {

		for i:=0; ;i++{
			var ICMPMessage4, ICMPMessage6 icmp.Message
			var ICMPEchoRequest []byte
			ICMPEchoResponse := make([]byte, 1500)
			var receivedFromAddress net.Addr
			start := time.Now()
			if *ipVersion6 {
				ICMPMessage6 = icmp.Message{
					Type:     ipv6.ICMPTypeEchoRequest,
					Code:     0,
					Body:     &icmp.Echo{
						ID:   20000,
						Seq:  i,
						Data: []byte("REQUEST"),
					},
				}
				ICMPEchoRequest, err = ICMPMessage6.Marshal(nil)
				if err != nil {
					panic(err)
				}
				_, err = connection6.WriteTo(ICMPEchoRequest, destinationAddress)
				if err != nil {
					fmt.Println(err)
					os.Exit(0)
				} else {
					stats.sentPackets ++
				}

				connection6.SetReadDeadline(start.Add(5 * time.Second))
				_,receivedFromAddress,err = connection6.ReadFrom(ICMPEchoResponse)
				if err != nil {
					fmt.Println("err")
					continue
				}

			} else {
				ICMPMessage4 = icmp.Message{
					Type:     ipv4.ICMPTypeEcho,
					Code:     0,
					Body:     &icmp.Echo{
						ID:   20000,
						Seq:  i,
						Data: []byte("REQUEST"),
					},
				}
				ICMPEchoRequest, err = ICMPMessage4.Marshal(nil)
				if err != nil {
					panic(err)
				}
				_, err = connection4.WriteTo(ICMPEchoRequest, destinationAddress)
				if err != nil {
					fmt.Println(err)
					os.Exit(0)
				} else {
					stats.sentPackets ++
				}

				connection4.SetReadDeadline(start.Add(5 * time.Second))
				_,receivedFromAddress,err = connection4.ReadFrom(ICMPEchoResponse)
				if err != nil {
					fmt.Println("err")
					continue
				}

			}

			duration := time.Since(start)
			if stats.minRTT > duration {
				stats.minRTT = duration
			}
			if stats.maxRTT < duration {
				stats.maxRTT = duration
			}
			stats.totalTime += duration



			if *ipVersion6 {
				message , err := icmp.ParseMessage(58, ICMPEchoResponse)
				if err != nil {
					panic(err)
				}
				if message.Type == ipv6.ICMPTypeDestinationUnreachable {
					switch message.Code {
					case 1:
						fmt.Printf("From %s icmp_seq=%d Destination Host Unreachable\n", receivedFromAddress, i+1)
					}
				} else if message.Type == ipv6.ICMPTypeEchoReply {
					switch message.Code {
					case 0:
						fmt.Printf("%d bytes from %s: icmp_seq= %d time=%v \n",cap(ICMPEchoRequest), receivedFromAddress, i+1, duration)
						stats.receivedPackets++
						done = true
					}
				}
			} else {
				message , err := icmp.ParseMessage(1, ICMPEchoResponse)
				if err != nil {
					panic(err)
				}
				if message.Type == ipv4.ICMPTypeDestinationUnreachable {
					switch message.Code {
					case 1:
						fmt.Printf("From %s icmp_seq=%d Destination Host Unreachable\n", receivedFromAddress, i+1)
					}
				} else if message.Type == ipv4.ICMPTypeEchoReply {
					switch message.Code {
					case 0:
						fmt.Printf("%d bytes from %s: icmp_seq= %d time=%v \n",cap(ICMPEchoRequest), receivedFromAddress, i+1, duration)
						stats.receivedPackets++
						done = true
					}

				}
			}
			time.Sleep(1 * time.Second - time.Since(start))
		}
	}()

	<-sigs
	fmt.Println("--- ping stats ---")
	percentLoss := (1 - (float32(stats.receivedPackets) / float32(stats.sentPackets))) * 100
	fmt.Printf("%d packets transmitted, %d recieved, %f%% packet loss, time %v \n",
		stats.sentPackets, stats.receivedPackets, percentLoss, time.Since(begin))
	if done {
		stats.avgRTT = stats.totalTime / 2
		fmt.Printf("rtt min/avg/max = %v/%v/%v \n", stats.minRTT, stats.avgRTT, stats.maxRTT)
	}
}
