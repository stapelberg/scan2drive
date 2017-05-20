// vim:ts=4:sw=4:et
// Documentation references:
// https://developers.google.com/identity/sign-in/web/reference
// https://developers.google.com/picker/docs/reference

var user;

function start() {
    console.log('start');

    gapi.load('auth2', function() {
        // TODO: avoid a global variable here.
        auth2 = gapi.auth2.init({
            client_id: clientID,
            // The “profile” and “email” scope are always requested.
            scope: 'https://www.googleapis.com/auth/drive',
        });
        auth2.then(function() {
            user = auth2.currentUser.get();
            var sub = $('#user-name').attr('data-sub');
            // If sub does not match the user id, we are logged in in the
            // browser, but not on the server side (e.g. because sessions were
            // deleted).
            if (auth2.isSignedIn.get() && user.getId() === sub) {
                $('#user-avatar').attr('src', user.getBasicProfile().getImageUrl());
                $('#user-name').text(user.getBasicProfile().getName());
                $('paper-fab').show();
                $('#signin').hide();
                $('#signout').show();
                $('#settings-button').show();
                // TODO: open settings button in case drive folder is not configured
            }
        }, function(err) {
	    var errorp = $('#error p');
	    errorp.text('Error ' + err.error + ': ' + err.details);
	    console.log('OAuth2 error', err);
        });
    });

    gapi.load('picker', function() {});

    $('#signinButton').click(function() {
        auth2.grantOfflineAccess({'redirect_uri': 'postmessage'}).then(signInCallback);
    });

    $('#signout').click(function(ev) {
        var auth2 = gapi.auth2.getAuthInstance();
        auth2.signOut().then(function() {
            $.ajax({
                type: 'POST',
                url: '/signout',
                success: function(result) {
                    // Reload the page.
                    window.location.href = window.location.href;
                },
                // TODO: error handling (deleting file failed, e.g. because of readonly file system)
            });
        });
        ev.preventDefault();
    });

    // TODO: #signin keypress
    $('#signin').click(function(ev) {
        auth2.grantOfflineAccess({'redirect_uri': 'postmessage'}).then(signInCallback);
        ev.preventDefault();
    });

    $('#select-drive-folder').click(function() {
        createPicker();
    });
}

function pollScan(name) {
    $.ajax({
        type: 'POST',
        url: '/scanstatus',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            $('#scan-progress-status').text(data.Status);
            console.log('result', data, 'textStatus', textStatus, 'jqXHR', jqXHR);
            if (jqXHR.status !== 200) {
                // TODO: show error message
                return;
            }
            if (data.Done) {
                $('#scan-dialog paper-spinner-lite').attr('active', null);
                $('paper-fab').attr('icon', 'hardware:scanner').attr('disabled', null);
                $('#scan-dialog').off('iron-overlay-canceled');
                var sub = user.getBasicProfile().getId();
                $('#scan-dialog .scan-thumb').css('background', 'url("scans_dir/' + sub + '/' + name + '/thumb.png")').css('background-size', 'cover');
            } else {
                setTimeout(function() { pollScan(name); }, 500);
            }
        },
        error: function(jqXHR, textStatus, errorThrown) {
            if (jqXHR.status === 404) {
                // Scan was not yet found because the directory rescan isn’t done.
                // Retry in a little while.
                setTimeout(function() { pollScan(name); }, 500);
            } else {
                $('#scan-progress-status').text('Error: ' + errorThrown);
                setTimeout(function() { pollScan(name); }, 500);
            }
        },
        processData: false,
        data: JSON.stringify({'Name':name}),
    });
}

function renameScan(name, newSuffix) {
    var newName = name + '-' + newSuffix;

    $.ajax({
        type: 'POST',
        url: '/renamescan',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            $('#scan-form paper-input iron-icon').show();
        },
        error: function(jqXHR, textStatus, errorThrown) {
            console.log(textStatus);
        },
        processData: false,
        data: JSON.stringify({
            Name: name,
            NewName: newName,
        }),
    });
}

function scan() {
    // Only one scan can be in progress at a time.
    $('paper-fab').attr('icon', 'hourglass-empty').attr('disabled', true);

    document.getElementById('scan-dialog').open();
    $('#scan-dialog').on('iron-overlay-canceled', function(ev) {
        // TODO: move status to toolbar instead of blocking close
        ev.preventDefault();
    });

    $.ajax({
        type: 'POST',
        url: '/startscan',
        success: function(data, textStatus, jqXHR) {
            $('#scan-dialog paper-input[name="name"] div[prefix]').text(data.Name + '-');
            var renameButton = $('#scan-form paper-button');
            renameButton.click(function(ev) {
                renameScan(data.Name, $('#scan-form paper-input').val());
            });
            pollScan(data.Name);
        },
        error: function(jqXHR, textStatus, errorThrown) {
            var exitStatus = jqXHR.getResponseHeader('X-Exit-Status');
            if (exitStatus === undefined) {
                console.log('TODO: error handler for non-scanimage internal server error');
                return;
            }
            // From sane-backends-1.0.25/include/sane/sane.h
            // SANE_STATUS_GOOD = 0,   /* everything A-OK */
            // SANE_STATUS_UNSUPPORTED,    /* operation is not supported */
            // SANE_STATUS_CANCELLED,  /* operation was cancelled */
            // SANE_STATUS_DEVICE_BUSY,    /* device is busy; try again later */
            // SANE_STATUS_INVAL,      /* data is invalid (includes no dev at open) */
            // SANE_STATUS_EOF,        /* no more data available (end-of-file) */
            // SANE_STATUS_JAMMED,     /* document feeder jammed */
            // SANE_STATUS_NO_DOCS,    /* document feeder out of documents */
            // SANE_STATUS_COVER_OPEN, /* scanner cover is open */
            // SANE_STATUS_IO_ERROR,   /* error during device I/O */
            // SANE_STATUS_NO_MEM,     /* out of memory */
            // SANE_STATUS_ACCESS_DENIED   /* access to resource has been denied */

            // TODO: butter bar with human-readable error code
            console.log('error ', exitStatus);
        },
    });
}

// callback has “loaded”, “cancel” and “picked”
function pickerCallback(data) {
    console.log('picker callback', data);
    if (data.action !== google.picker.Action.PICKED) {
        return;
    }
    if (data.docs.length !== 1) {
        // TODO: error handling
        return;
    }
    var picked = data.docs[0];
    // TODO: show a spinner
    $.ajax({
        type: 'POST',
        url: '/storedrivefolder',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            console.log('result', data, 'textStatus', textStatus, 'jqXHR', jqXHR);
            if (jqXHR.status !== 200) {
                // TODO: show error message
                return;
            }
            $('#drive-folder').html($('<img>').attr('src', picked.iconUrl).css('margin-right', '0.25em')).append($('<a />').attr('href', picked.url).text(picked.name));
        },
        data: JSON.stringify({
            'Id': picked.id,
            'IconUrl': picked.iconUrl,
            'Url': picked.url,
            'Name': picked.name,
        }),
    });
}

function createPicker() {
    if (!user) {
        // The picker requires an OAuth token.
        return;
    }

    var docsView = new google.picker.DocsView()
        .setIncludeFolders(true)
        .setMimeTypes('application/vnd.google-apps.folder')
        .setMode(google.picker.DocsViewMode.LIST)
        .setSelectFolderEnabled(true);

    var picker = new google.picker.PickerBuilder()
        .addView(docsView)
        .setCallback(pickerCallback)
        .setOAuthToken(user.getAuthResponse().access_token)
        .build();
    picker.setVisible(true);
}

function signInCallback(authResult) {
    if (authResult['code']) {
        // TODO: progress indicator, writing to disk and examining scans could take a while.
        $.ajax({
            type: 'POST',
            url: '/oauth',
            contentType: 'application/octet-stream; charset=utf-8',
            success: function(result) {
                // Reload the page.
                window.location.href = window.location.href;
            },
            error: function(jqXHR, textStatus, errorThrown) {
		if (jqXHR.status == 500) {
                    $('#error p').text('OAuth error: ' + jqXHR.responseText + '. Try revoking access on https://security.google.com/settings/u/0/security/permissions, then retry');
		} else {
                    $('#error p').text('Unknown OAuth error: ' + errorThrown);
                }
            },
            processData: false,
            data: authResult['code'],
        });
    } else {
        console.log('sth went wrong :|', authResult);
        // TODO: trigger logout, without server-side auth we are screwed
    }
}
