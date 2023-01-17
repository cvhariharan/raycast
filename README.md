# Raycast
Turn your phone into a webcam, wirelessly.   
Raycast uses the mediadevices API to capture phone's camera and transmits it via WebRTC to a server on the PC which sends it to a virtual camera created using [v4l2loopback](https://github.com/umlaeute/v4l2loopback).

## Dependencies
- v4l2loopback
- FFmpeg
- Go
- Node

## Build
Create a virtual camera
```sh
modprobe v4l2loopback video_nr=10 exclusive_caps=1 card_label="VirtualCam"
```

Browsers allow camera access only under HTTPS. Create self-signed certs by running
```sh
chmod +x makecert.sh
./makecert.sh
```

Launch the application
```sh
make server
```