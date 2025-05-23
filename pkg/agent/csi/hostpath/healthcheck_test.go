/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

const (
	originalMountInfo = `{
    "filesystems": [
        {
            "target": "/",
            "source": "overlay",
            "fstype": "overlay",
            "options": "rw,relatime,lowerdir=/var/lib/docker/overlay2/l/472T4X42Q446EJ2QSUSNNNI7DJ:/var/lib/docker/overlay2/l/RCFJKQWDXJNO26LYOBWHZKGOJK:/var/lib/docker/overlay2/l/L7PB4C6IPA6RQ4SSZWDHCFM74H:/var/lib/docker/overlay2/l/MVHKUEXU6I2CJJOKDIZ56YJ5PI,upperdir=/var/lib/docker/overlay2/910d3fc528c6e7db604b65e4120cac71a35f4be969113bf07181ebf440561222/diff,workdir=/var/lib/docker/overlay2/910d3fc528c6e7db604b65e4120cac71a35f4be969113bf07181ebf440561222/work",
            "children": [
                {
                    "target": "/proc",
                    "source": "proc",
                    "fstype": "proc",
                    "options": "rw,nosuid,nodev,noexec,relatime"
                },
                {
                    "target": "/sys",
                    "source": "sysfs",
                    "fstype": "sysfs",
                    "options": "ro,nosuid,nodev,noexec,relatime",
                    "children": [
                        {
                            "target": "/sys/fs/cgroup",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,nosuid,nodev,noexec,relatime,mode=755",
                            "children": [
                                {
                                    "target": "/sys/fs/cgroup/systemd",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,xattr,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd"
                                },
                                {
                                    "target": "/sys/fs/cgroup/cpu,cpuacct",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,cpuacct,cpu"
                                },
                                {
                                    "target": "/sys/fs/cgroup/devices",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,devices"
                                },
                                {
                                    "target": "/sys/fs/cgroup/memory",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,memory"
                                },
                                {
                                    "target": "/sys/fs/cgroup/pids",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,pids"
                                },
                                {
                                    "target": "/sys/fs/cgroup/net_cls,net_prio",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,net_prio,net_cls"
                                },
                                {
                                    "target": "/sys/fs/cgroup/perf_event",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,perf_event"
                                },
                                {
                                    "target": "/sys/fs/cgroup/freezer",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,freezer"
                                },
                                {
                                    "target": "/sys/fs/cgroup/hugetlb",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,hugetlb"
                                },
                                {
                                    "target": "/sys/fs/cgroup/cpuset",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,cpuset"
                                },
                                {
                                    "target": "/sys/fs/cgroup/blkio",
                                    "source": "cgroup[/kubepods/besteffort/podfc988f20-10dd-11eb-96e3-fa163f04be9d/543493900fd363057933892062d0bc0cc8a54e30d137ec2bb0027977f3ac73ea]",
                                    "fstype": "cgroup",
                                    "options": "rw,nosuid,nodev,noexec,relatime,blkio"
                                }
                            ]
                        }
                    ]
                },
                {
                    "target": "/csi",
                    "source": "/dev/vda2[/var/lib/kubelet/plugins/csi-hostpath]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/csi-data-dir",
                    "source": "/dev/vda2[/var/lib/csi-hostpath-data]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/dev",
                    "source": "devtmpfs",
                    "fstype": "devtmpfs",
                    "options": "rw,nosuid,size=3993932k,nr_inodes=998483,mode=755",
                    "children": [
                        {
                            "target": "/dev/shm",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,nosuid,nodev",
                            "children": [
                                {
                                    "target": "/dev/shm",
                                    "source": "shm",
                                    "fstype": "tmpfs",
                                    "options": "rw,nosuid,nodev,noexec,relatime,size=65536k"
                                }
                            ]
                        },
                        {
                            "target": "/dev/pts",
                            "source": "devpts",
                            "fstype": "devpts",
                            "options": "rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000"
                        },
                        {
                            "target": "/dev/hugepages",
                            "source": "hugetlbfs",
                            "fstype": "hugetlbfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/dev/mqueue",
                            "source": "mqueue",
                            "fstype": "mqueue",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/dev/termination-log",
                            "source": "/dev/vda2[/var/lib/kubelet/pods/fc988f20-10dd-11eb-96e3-fa163f04be9d/containers/hostpath/d1c9c9bc]",
                            "fstype": "xfs",
                            "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                        }
                    ]
                },
                {
                    "target": "/etc/resolv.conf",
                    "source": "/dev/vda2[/var/lib/docker/containers/9da684d4bf029592568b82e90a707c82e6a02d81585344bd9107215c937a3abc/resolv.conf]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/etc/hostname",
                    "source": "/dev/vda2[/var/lib/docker/containers/9da684d4bf029592568b82e90a707c82e6a02d81585344bd9107215c937a3abc/hostname]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/etc/hosts",
                    "source": "/dev/vda2[/var/lib/kubelet/pods/fc988f20-10dd-11eb-96e3-fa163f04be9d/etc-hosts]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/var/lib/kubelet/pods",
                    "source": "/dev/vda2[/var/lib/kubelet/pods]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota",
                    "children": [
                        {
                            "target": "/var/lib/kubelet/pods/b5d6777b-12aa-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/persistent-volume-csi-cinder-node-sa-token-d95br",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/b5d6777b-12aa-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/secret-cinderplugin",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/be4f605f-1293-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-js9bk",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/bb6265c4-14d5-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/cattle-token-fg68p",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/bb6265c4-14d5-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/cattle-credentials",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/61d7c25c-150b-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-js9bk",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/3440e8ee-10de-11eb-8895-fa163feebd84/volumes/kubernetes.io~secret/default-token-8vw47",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/3440e8ee-10de-11eb-8895-fa163feebd84/volumes/kubernetes.io~csi/pvc-33d023c7-10de-11eb-8895-fa163feebd84/mount",
                            "source": "/dev/vda2[/var/lib/csi-hostpath-data/39267558-10de-11eb-8fb9-0a58ac120605]",
                            "fstype": "xfs",
                            "options": "rw,noatime,attr2,nobarrier,inode64,noquota"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/6040ee58-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/cluster-monitoring-node-exporter-token-lfkrv",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/60433639-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-js9bk",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/60403c28-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-js9bk",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/60459ef2-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/flannel-token-j64bp",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/604b15ec-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-js9bk",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/604b08d5-0d0c-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/vksdns-token-d9lnm",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/fc988f20-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-8vw47",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/fd31fc4b-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/csi-provisioner-token-xbdnl",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/b5a70e65-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/csi-attacher-token-wmm5s",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/fdd58e3d-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/csi-resizer-token-zpgnc",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/fe7e95e6-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/csi-snapshotter-token-sz4wc",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        },
                        {
                            "target": "/var/lib/kubelet/pods/ff4a82b1-10dd-11eb-96e3-fa163f04be9d/volumes/kubernetes.io~secret/default-token-8vw47",
                            "source": "tmpfs",
                            "fstype": "tmpfs",
                            "options": "rw,relatime"
                        }
                    ]
                },
                {
                    "target": "/var/lib/kubelet/plugins",
                    "source": "/dev/vda2[/var/lib/kubelet/plugins]",
                    "fstype": "xfs",
                    "options": "rw,nodev,noatime,attr2,nobarrier,inode64,noquota"
                },
                {
                    "target": "/run/secrets/kubernetes.io/serviceaccount",
                    "source": "tmpfs",
                    "fstype": "tmpfs",
                    "options": "ro,relatime"
                }
            ]
        }
    ]
}`
)

func TestParseMountInfo(t *testing.T) {
	containerFileSystems, err := parseMountInfo([]byte(originalMountInfo))
	assert.Nil(t, err)
	assert.NotNil(t, containerFileSystems)
}

func TestFilterVolumeName(t *testing.T) {
	targetPath := "/var/lib/kubelet/pods/3440e8ee-10de-11eb-8895-fa163feebd84/volumes/kubernetes.io~csi/pvc-33d023c7-10de-11eb-8895-fa163feebd84/mount"
	volumeName := filterVolumeName(targetPath)
	assert.Equal(t, "pvc-33d023c7-10de-11eb-8895-fa163feebd84", volumeName)
}

func TestFilterVolumeID(t *testing.T) {
	sourcePath := "/dev/vda2[/var/lib/csi-hostpath-data/39267558-10de-11eb-8fb9-0a58ac120605]"
	volumeID := filterVolumeID(sourcePath)
	assert.Equal(t, "39267558-10de-11eb-8fb9-0a58ac120605", volumeID)
}

func filterVolumeName(targetPath string) string {
	pathItems := strings.Split(targetPath, "kubernetes.io~csi/")
	if len(pathItems) < 2 {
		return ""
	}

	return strings.TrimSuffix(pathItems[1], "/mount")
}

func filterVolumeID(sourcePath string) string {
	volumeSourcePathRegex := regexp.MustCompile(`\[(.*)\]`)
	volumeSP := string(volumeSourcePathRegex.Find([]byte(sourcePath)))
	if volumeSP == "" {
		return ""
	}

	return strings.TrimSuffix(strings.TrimPrefix(volumeSP, "[/var/lib/csi-hostpath-data/"), "]")
}

func TestCheckPathExist(t *testing.T) {
	pathA, err := os.MkdirTemp("/tmp", "ioiTest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(pathA)
	exist, _ := checkPathExist(pathA)
	assert.True(t, exist)

	exist, _ = checkPathExist(filepath.Join("/tmp", "ioiTest", "nonexist"))
	assert.False(t, exist)
}

func TestGetPVStats(t *testing.T) {
	_, _, _, _, _, _, err := getPVStats("/")
	assert.Nil(t, err)
}

func TestCheckPVUsage(t *testing.T) {
	err := os.Mkdir("/tmp/pvtest", os.ModeDir)
	if err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	defer os.RemoveAll("/tmp/pvtest")
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		config: Config{
			StateDir: "/tmp",
		},
		state: s,
	}
	volume := state.Volume{
		VolID:   "pvtest",
		VolName: "pvtest",
		VolPath: "/tmp/pvtest",
		VolSize: 1,
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args string
		want bool
	}{
		{
			name: "path exists",
			args: "pvtest",
			want: true,
		},
		{
			name: "path does not exist",
			args: "pvtest1",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := hp.checkPVUsage(tt.args)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("healthcheck.checkPVUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPVCapacityValid(t *testing.T) {
	err := os.Mkdir("/tmp/pvcaptest", os.ModeDir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("/tmp/pvcaptest")

	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		config: Config{
			StateDir: "/tmp",
		},
		state: s,
	}

	volume := state.Volume{
		VolID:   "pvcaptest",
		VolName: "pvcaptest",
		VolPath: "/tmp/pvcaptest",
		VolSize: 1,
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path.Join("/tmp", "state.json"))

	tests := []struct {
		name string
		args string
		want bool
	}{
		{
			name: "path does not exists",
			args: "pvcaptest1",
			want: false,
		},
		{
			name: "volume id does not exist",
			args: "pvcaptest",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := hp.checkPVCapacityValid(tt.args)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("healthcheck.checkPVCapacityValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoHealthCheckInControllerSide(t *testing.T) {
	err := os.Mkdir("/tmp/hctest", os.ModeDir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("/tmp/hctest")
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		config: Config{
			StateDir: "/tmp",
		},
		state: s,
	}

	volume := state.Volume{
		VolID:   "hctest",
		VolName: "hctest",
		VolPath: "/tmp/hctest",
		VolSize: 1,
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path.Join("/tmp", "state.json"))

	tests := []struct {
		name   string
		args   string
		want   bool
		errMsg string
	}{
		{
			name:   "path does not exist",
			args:   "hctest1",
			want:   false,
			errMsg: "cannot get volume path",
		},
		{
			name:   "success",
			args:   "hctest",
			want:   true,
			errMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hp.doHealthCheckInControllerSide(tt.args)
			if len(err) > 0 && !strings.Contains(err, tt.errMsg) {
				t.Errorf("healthcheck.doHealthCheckInControllerSide() error = %v, wantErr %v", err, tt.errMsg)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("healthcheck.doHealthCheckInControllerSide() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoHealthCheckInNodeSide(t *testing.T) {
	patches := gomonkey.ApplyFunc(checkMountPointExist, func(pth string) (bool, string) {
		if pth == "/tmp/hcntest" {
			return true, ""
		} else {
			return false, ""
		}
	})
	defer patches.Reset()

	err := os.Mkdir("/tmp/hcntest", os.ModeDir)
	if err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	defer os.RemoveAll("/tmp/hcntest")
	s, err := state.New(path.Join("/tmp", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hp := &hostPath{
		config: Config{
			StateDir: "/tmp",
		},
		state: s,
	}
	volume := state.Volume{
		VolID:   "hcntest",
		VolName: "hcntest",
		VolPath: "/tmp/hcntest",
		VolSize: 1,
	}
	if err := hp.state.UpdateVolume(volume); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		args   string
		want   bool
		errMsg string
	}{
		{
			name:   "path exists",
			args:   "hcntest",
			want:   true,
			errMsg: "",
		},
		{
			name:   "path does not exist",
			args:   "pvtest1",
			want:   false,
			errMsg: "cannot get volume path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hp.doHealthCheckInNodeSide(tt.args)
			if len(err) > 0 && !strings.Contains(err, tt.errMsg) {
				t.Errorf("healthcheck.doHealthCheckInNodeSide() error = %v, wantErr %v", err, tt.errMsg)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("healthcheck.doHealthCheckInNodeSide() = %v, want %v", got, tt.want)
			}
		})
	}
}
