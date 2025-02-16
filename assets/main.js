"use strict";

document.addEventListener("DOMContentLoaded", () => {
    const handleRegex = /^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/;
    const dhRegex = /^dh=[0-9a-f]{40}$/;

    const registrationForm = document.getElementById("registrationForm");
    const oldHandle = document.getElementById("oldHandle");
    const newHandle = document.getElementById("newHandle");
    const dhCode = document.getElementById("dhCode");
    const msgText = document.getElementById("msgText");

    registrationForm.addEventListener("submit", async (event) => {
        event.preventDefault();

        msgText.classList.add("noDisplay");
        msgText.classList.remove("errorText");
        msgText.textContent = "";

        try {
            if (!handleRegex.test(newHandle.value)) {
                throw new Error("New handle is invalid");
            }

            if (dhCode.value != "" && !dhRegex.test(dhCode.value)) {
                throw new Error("DH Code is invalid");
            }

            const bAPI = await fetch("https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor="+oldHandle.value);
            if (bAPI.status != 200) {
                throw new Error("Failed to find account with the current handle");
            }

            const bJSON = await bAPI.json();
            if (!bJSON.did) {
                throw new Error("Unable to find the DID for the account with the current handle");
            }

            const rfData = new FormData(registrationForm);
            rfData.set("didHelper", bJSON.did);

            const vAPI = await fetch("https://synth.club/regVerify", {
                method: "POST",
                body: rfData
            });

            if (vAPI.status == 429) {
                throw new Error("Slow down, wait a few seconds... (Too many requests)");
            } else if (vAPI.status != 200) {
                throw new Error(`There was an error while verifying... (Status: ${vAPI.status})`);
            }

            const vJSON = await vAPI.json();
            if (!vJSON.isSuccess) {
                throw new Error(vJSON.errorMessage);
            }

            registrationForm.submit();
        } catch (err) {
            msgText.classList.add("errorText");
            msgText.classList.remove("noDisplay");
            msgText.textContent = err.message;
        }
    })
})