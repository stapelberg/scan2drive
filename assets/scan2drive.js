// vim:ts=4:sw=4:et
// Documentation references:
// https://developers.google.com/identity/sign-in/web/reference
// https://developers.google.com/picker/docs/reference

function httpErrorToToast(jqXHR, prefix) {
    var summary = 'HTTP ' + jqXHR.status + ': ' + jqXHR.responseText;
    Materialize.toast(prefix + ': ' + summary, 5000, 'red');
    console.log('error: prefix', prefix, ', summary', summary);
}

// start is called once the Google APIs were loaded
function start() {
    console.log('start');

    gapi.load('auth2', function() {
        var auth2 = gapi.auth2.init({
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
		console.log('logged in');
                $('#user-avatar').attr('src', user.getBasicProfile().getImageUrl());
                $('#user-name').text(user.getBasicProfile().getName());
                $('.fixed-action-btn').show();
                $('#signin').hide();
                $('#signout').show();
                $('#settings-button').show();
                // TODO: open settings button in case drive folder is not configured
            } else {
		console.log('auth2 loaded, but user not logged in');
	    }
        }, function(err) {
	    var errorp = $('#error p');
	    errorp.text('Error ' + err.error + ': ' + err.details);
	    console.log('OAuth2 error', err);
        });
    });

    gapi.signin2.render('my-signin2', {
        'scope': 'profile email',
        'width': 240,
        'height': 50,
        'longtitle': true,
        'theme': 'dark',
        'onsuccess': function(){ console.log('success'); },
        'onfailure': function() { console.log('failure'); }
    });

    gapi.load('picker', function() {});

    $('#signinButton').click(function() {
	var auth2 = gapi.auth2.getAuthInstance();
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
                    window.location.href = window.location.origin;
                },
                // TODO: error handling (deleting file failed, e.g. because of readonly file system)
            });
        });
        ev.preventDefault();
    });

    // TODO: #signin keypress
    $('#signin').click(function(ev) {
	var auth2 = gapi.auth2.getAuthInstance();
        auth2.grantOfflineAccess({'redirect_uri': 'postmessage'}).then(signInCallback);
        ev.preventDefault();
    });

    $('#select-drive-folder').click(function() {
        createPicker();
    });

    // Resolve user ids into names and thumbnails for the people dialog
    $('div.user').each(function(idx, el) {
	var sub = $(el).data('sub');
	$.ajax({
	    url: 'https://picasaweb.google.com/data/entry/api/user/' + sub + '?alt=json',
	    success: function(result) {
		var nick = result.entry.gphoto$nickname.$t;
		var thumb = result.entry.gphoto$thumbnail.$t;
		$('div.user[data-sub="' + sub + '"] img').attr('src', thumb);
		$('div.user[data-sub="' + sub + '"] span.user-nick').text(nick);
	    },
	});
    });

    $('div.user').click(function() {
	var sub = $(this).data('sub');
	// TODO: show progress spinner
	$.ajax({
            type: 'POST',
            url: '/storedefaultuser',
            contentType: 'application/json',
            success: function(data, textStatus, jqXHR) {
		$('div.user').removeClass('default-user');
		$('div.user[data-sub="' + sub + '"]').addClass('default-user');
		$('#people-dialog').modal('close');
            },
            error: function(jqXHR, textStatus, errorThrown) {
		httpErrorToToast(jqXHR, 'storing default user failed');
            },
            processData: false,
            data: JSON.stringify({
		DefaultSub: sub,
            }),
	});
    });
}

function pollScan(name) {
    var user = gapi.auth2.getAuthInstance().currentUser.get();
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
                $('#scan-dialog paper-spinner-lite').attr('active', null); // TODO
                $('.fixed-action-btn i').text('scanner');
                $('.fixed-action-btn a').removeClass('disabled');
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
            httpErrorToToast(jqXHR, 'renaming scan failed');
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
    $('.fixed-action-btn i').text('hourglass_empty');
    $('.fixed-action-btn a').addClass('disabled');
    $('#scan-dialog').modal('open');

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
            $('#scan-dialog').modal('close');
            $('.fixed-action-btn i').text('scanner');
            $('.fixed-action-btn a').removeClass('disabled');
            httpErrorToToast(jqXHR, 'scanning failed');
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
            $('#drivefolder').val(picked.name);
        },
        error: function(jqXHR, textStatus, errorThrown) {
            httpErrorToToast(jqXHR, 'storing drive folder failed');
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
    var user = gapi.auth2.getAuthInstance().currentUser.get();

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
                window.location.href = window.location.origin;
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
