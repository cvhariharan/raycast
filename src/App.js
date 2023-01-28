import React, { useEffect, useRef, useState } from "react";
import VideoPlayer from "./VideoPlayer";
import { Buffer } from 'buffer';

const App = () => {
    const [message, setMessage] = useState('')
    const [offer, setOffer] = useState('')
    const [remote, setRemote] = useState('')
    let video = useRef()
    
    let peer = useRef(new RTCPeerConnection())

    const getMedia = async () => { 
        let stream = null;
        try {
            stream = await navigator.mediaDevices.getUserMedia({
                video: {
                    facingMode: "user",
                    frameRate: 30,
                    height: 480,
                    width: 640
                }
            })
        } catch(err) {
            console.log(err)
            setMessage(err.toString())
        }

        return stream
    }

    useEffect(() => {
        async function getStream() {
            const media = await getMedia()
            video.current.srcObject = media
            video.current.onloadedmetadata = (e) => { video.current.play() }
            
            for(const track of media.getTracks()) {
                peer.current.addTrack(track, media)
            }

            peer.current.createOffer().then(d => peer.current.setLocalDescription(d)).catch(console.log)

            peer.current.onicecandidate = ({candidate}) => {
                if(candidate === null) {
                    console.log(peer.current.localDescription)
                    setOffer(Buffer.from(JSON.stringify(peer.current.localDescription)).toString('base64'))
                    console.log(window.location.hostname)
                    fetch("/sdp", {
                        method: "post",
                        headers: {
                            'Accept': 'application/json',
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({
                            sdp: Buffer.from(JSON.stringify(peer.current.localDescription)).toString('base64')
                        })
                    }).then(async (resp) => {
                        let response = await resp.json()
                        peer.current.setRemoteDescription(JSON.parse(atob(response.sdp)))
                    })
                }
            }
        }
        getStream()
    }, [])

    const getRemoteConnection = (e) => {
        console.log(e.target.value)
        setRemote(e.target.value)
    }

    const setAnswer = (e) => {
        peer.current.setRemoteDescription(JSON.parse(atob(remote)))
    }

    return ( <div>
        <VideoPlayer video={video}/>
        {
            message !== '' ? 
            <p>Error: {message}</p> :
            <p>No error</p>
        }
        <textarea rows="4" cols="50" defaultValue={offer}></textarea>
        <textarea rows="4" cols="50" onChange={getRemoteConnection}></textarea>
        <button onClick={setAnswer}>Set</button>
    </div> )
}

export default App