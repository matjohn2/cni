// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ns_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/appc/cni/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getInode(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return getInodeF(file)
}

func getInodeF(file *os.File) (uint64, error) {
	stat := &unix.Stat_t{}
	err := unix.Fstat(int(file.Fd()), stat)
	return stat.Ino, err
}

const CurrentNetNS = "/proc/self/ns/net"

var _ = Describe("Linux namespace operations", func() {
	Describe("WithNetNS", func() {
		var (
			originalNetNS *os.File

			targetNetNSName string
			targetNetNSPath string
			targetNetNS     *os.File
		)

		BeforeEach(func() {
			var err error
			originalNetNS, err = os.Open(CurrentNetNS)
			Expect(err).NotTo(HaveOccurred())

			targetNetNSName = fmt.Sprintf("test-netns-%d", rand.Int())

			err = exec.Command("ip", "netns", "add", targetNetNSName).Run()
			Expect(err).NotTo(HaveOccurred())

			targetNetNSPath = filepath.Join("/var/run/netns/", targetNetNSName)
			targetNetNS, err = os.Open(targetNetNSPath)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(targetNetNS.Close()).To(Succeed())

			err := exec.Command("ip", "netns", "del", targetNetNSName).Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(originalNetNS.Close()).To(Succeed())
		})

		It("executes the callback within the target network namespace", func() {
			expectedInode, err := getInode(targetNetNSPath)
			Expect(err).NotTo(HaveOccurred())

			var actualInode uint64
			var innerErr error
			err = ns.WithNetNS(targetNetNS, false, func(*os.File) error {
				actualInode, innerErr = getInode(CurrentNetNS)
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(innerErr).NotTo(HaveOccurred())
			Expect(actualInode).To(Equal(expectedInode))
		})

		It("provides the original namespace as the argument to the callback", func() {
			hostNSInode, err := getInode(CurrentNetNS)
			Expect(err).NotTo(HaveOccurred())

			var inputNSInode uint64
			var innerErr error
			err = ns.WithNetNS(targetNetNS, false, func(inputNS *os.File) error {
				inputNSInode, err = getInodeF(inputNS)
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(innerErr).NotTo(HaveOccurred())
			Expect(inputNSInode).To(Equal(hostNSInode))
		})

		It("restores the calling thread to the original network namespace", func() {
			preTestInode, err := getInode(CurrentNetNS)
			Expect(err).NotTo(HaveOccurred())

			err = ns.WithNetNS(targetNetNS, false, func(*os.File) error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			postTestInode, err := getInode(CurrentNetNS)
			Expect(err).NotTo(HaveOccurred())

			Expect(postTestInode).To(Equal(preTestInode))
		})

		Context("when the callback returns an error", func() {
			It("restores the calling thread to the original namespace before returning", func() {
				preTestInode, err := getInode(CurrentNetNS)
				Expect(err).NotTo(HaveOccurred())

				_ = ns.WithNetNS(targetNetNS, false, func(*os.File) error {
					return errors.New("potato")
				})

				postTestInode, err := getInode(CurrentNetNS)
				Expect(err).NotTo(HaveOccurred())

				Expect(postTestInode).To(Equal(preTestInode))
			})

			It("returns the error from the callback", func() {
				err := ns.WithNetNS(targetNetNS, false, func(*os.File) error {
					return errors.New("potato")
				})
				Expect(err).To(MatchError("potato"))
			})
		})

		Describe("validating inode mapping to namespaces", func() {
			It("checks that different namespaces have different inodes", func() {
				hostNSInode, err := getInode(CurrentNetNS)
				Expect(err).NotTo(HaveOccurred())

				testNsInode, err := getInode(targetNetNSPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(hostNSInode).NotTo(Equal(0))
				Expect(testNsInode).NotTo(Equal(0))
				Expect(testNsInode).NotTo(Equal(hostNSInode))
			})
		})
	})
})
