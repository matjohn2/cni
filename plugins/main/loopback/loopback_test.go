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

package main_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/appc/cni/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Loopback", func() {
	var (
		networkNS   string
		containerID string
		command     *exec.Cmd
		environ     []string
	)

	BeforeEach(func() {
		command = exec.Command(pathToLoPlugin)
		containerID = "some-container-id"
		networkNS = makeNetworkNS(containerID)

		environ = []string{
			fmt.Sprintf("CNI_CONTAINERID=%s", containerID),
			fmt.Sprintf("CNI_NETNS=%s", networkNS),
			fmt.Sprintf("CNI_IFNAME=%s", "this is ignored"),
			fmt.Sprintf("CNI_ARGS=%s", "none"),
			fmt.Sprintf("CNI_PATH=%s", "/some/test/path"),
		}
		command.Stdin = strings.NewReader("this doesn't matter")
	})

	AfterEach(func() {
		Expect(removeNetworkNS(networkNS)).To(Succeed())
	})

	Context("when given a network namespace", func() {
		It("sets the lo device to UP", func() {
			command.Env = append(environ, fmt.Sprintf("CNI_COMMAND=%s", "ADD"))

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gbytes.Say(`{.*}`))
			Eventually(session).Should(gexec.Exit(0))

			var lo *net.Interface
			err = ns.WithNetNSPath(networkNS, true, func(hostNS *os.File) error {
				var err error
				lo, err = net.InterfaceByName("lo")
				return err
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(lo.Flags & net.FlagUp).To(Equal(net.FlagUp))
		})

		It("sets the lo device to DOWN", func() {
			command.Env = append(environ, fmt.Sprintf("CNI_COMMAND=%s", "DEL"))

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gbytes.Say(``))
			Eventually(session).Should(gexec.Exit(0))

			var lo *net.Interface
			err = ns.WithNetNSPath(networkNS, true, func(hostNS *os.File) error {
				var err error
				lo, err = net.InterfaceByName("lo")
				return err
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(lo.Flags & net.FlagUp).NotTo(Equal(net.FlagUp))
		})
	})
})
