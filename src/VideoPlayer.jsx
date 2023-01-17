import React from "react";

const VideoPlayer = (props) => {
    return (
        <video ref={props.video} autoPlay playsInline muted></video>
    )
}

export default VideoPlayer