
		package main

		const (
			indexTemplate string = `{{define "layout"}}
			<!doctype html>
<html>
<head>
  <meta charset='utf-8'>
  <meta name='viewport' content='width=device-width, initial-scale=1, shrink-to-fit=no'>

  <title>First template</title>

  <link rel='stylesheet' href='https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/css/bootstrap.min.css' integrity='sha384-TX8t27EcRE3e/ihU7zmQxVncDAy5uIKz4rEkgIXeMed4M0jlfIDPvg6uqKI2xXr2' crossorigin='anonymous'>  <script src='https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta1/dist/js/bootstrap.bundle.min.js' integrity='sha384-ygbV9kiqUc6oa4msXn9868pTtWMgiQaeYH7/t7LECLbyPA2x65Kgf80OJFdroafW' crossorigin='anonymous'></script>
  <script src='https://code.jquery.com/jquery-3.5.1.min.js'></script>
  <script src='https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/js/bootstrap.bundle.min.js' integrity='sha384-ho+j7jyWK8fNQe+A12Hb8AhRq26LrZ/JpcUGGOn+Y7RsweNrtN/tE3MoK7ZeZDyx' crossorigin='anonymous'></script>
  <script src='https://kit.fontawesome.com/62eabbd844.js' crossorigin='anonymous'></script>

  <script>
    var gameList = JSON.parse({{.GamesList}});
    var config = JSON.parse({{.Config}});
    var cookiesList = JSON.parse({{.CookiesList}});
  </script>
  <style>
    body {
      margin: 25pt;
    }
  </style>
</head>
<body>
  <ul class='nav nav-tabs' id='cfgTabs' role='tablist'>
    <li class='nav-item' role='presentation'>
      <a class='nav-link active' id='games-tab' data-toggle='tab' href='#games' role='tab' aria-controls='games' aria-selected='true'>Games List</a>
    </li>
    <li class='nav-item' role='presentation'>
      <a class='nav-link' id='cookies-tab' data-toggle='tab' href='#cookies' role='tab' aria-controls='cookies' aria-selected='false'>Cookies</a>
    </li>
    <li class='nav-item' role='presentation'>
      <a class='nav-link' id='config-tab' data-toggle='tab' href='#config' role='tab' aria-controls='config' aria-selected='false'>Config</a>
    </li>
  </ul>

  <div class='tab-content'>
    <div class='tab-pane show active border border-top-0 rounded-bottom p-1' id='games' role='tabpanel' aria-labelledby='games-tab'>
      <div class='list-group' id='games-div'>
      </div>
      <p></p>
      <div class='input-group'>
        <input class='form-control form-control-lg' type='text' placeholder='Paste Steampage URL or #num:name' id='new-game-url'>
        <div class='input-group-append'>
          <button class='btn btn-success' type='button' id='button-add-game'><i class='fas fa-plus-circle'>&nbsp;</i>Add</button>
        </div>
      </div>
      <p></p>
      <div class="alert alert-success" role="alert" id="games-success-alert" hidden>
        Games List saved successfully!
      </div>
      <div class="alert alert-danger" role="alert" id="games-error-alert" hidden>
        Error while saving. See logs for details.
      </div>
      <button type='button' class='btn btn btn-primary' id='save-games'><i class='fas fa-cloud-upload-alt'>&nbsp;</i>&nbsp;Save games</button>
    </div>

    <div class='tab-pane border border-top-0 rounded-bottom p-1' id='cookies' role='tabpanel' aria-labelledby='cookies-tab'>
      <div class='list-group' id='cookies-div'>
      </div>
      <p></p>
      <div class='d-flex flex-row'>
        <input class='form-control form-control-lg w-25 mr-3' type='text' placeholder='Name' id='cookie-name'>
        <div class='input-group'>
          <input class='form-control form-control-lg' type='text' placeholder='Value' id='cookie-value'>
          <input class='form-control form-control-lg' type='text' placeholder='Domain' id='cookie-domain'>
          <input class='form-control form-control-lg' type='text' placeholder='Path' id='cookie-path'>
          <div class='input-group-append'>
            <button class='btn btn-success' type='button' id='button-add-cookie'><i class='fas fa-plus-circle'>&nbsp;</i>Add</button>
          </div>
        </div>
      </div>
      <p></p>
      <div class="alert alert-success" role="alert" id="cookies-success-alert" hidden>
        Cookies saved successfully!
      </div>
      <div class="alert alert-danger" role="alert" id="cookies-error-alert" hidden>
        Error while saving. See logs for details.
      </div>
      <button type='button' class='btn btn btn-primary' id='save-cookies'><i class='fas fa-cloud-upload-alt'>&nbsp;</i>&nbsp;Save cookies</button>
    </div>

    <div class='tab-pane border border-top-0 rounded-bottom p-1' id='config' role='tabpanel' aria-labelledby='config-tab'>
      <div class='list-group' id='config-div'>
      </div>
      <p></p>
      <div class="alert alert-warning" role="alert">
        Config changes will apply only after manual bot restart!
      </div>
      <div class="alert alert-success" role="alert" id="config-success-alert" hidden>
        Config saved successfully!
      </div>
      <div class="alert alert-danger" role="alert" id="config-error-alert" hidden>
        Error while saving. See logs for details.
      </div>
      <button type='button' class='btn btn btn-primary' id='save-config'><i class='fas fa-cloud-upload-alt'>&nbsp;</i>&nbsp;Save changes</button>
    </div>
  </div>
<script>
  Object.defineProperty(String.prototype, 'hashCode', {
    value: function() {
      var hash = 0, i, chr;
      for (i = 0; i < this.length; i++) {
        chr   = this.charCodeAt(i);
        hash  = ((hash << 5) - hash) + chr;
        hash |= 0; // Convert to 32bit integer
      }
      return hash;
    }
  });

  function RenderGamesList() {
    $("#game-edit").off();
    $("#game-remove").off();
    $("#button-edit-cancel").off();
    $("#button-edit-game").off();

    $('#games-div').empty();

    for (const g in gameList) {
      $('#games-div').append(
      "<li class='list-group-item bg-light'> \
        <span id='game-link-" + g + "'> \
          <a class='align-middle' href='https://store.steampowered.com/app/" + g + "/' target=blank> \
          <img class='rounded' height=43 src='https://steamcdn-a.akamaihd.net/steam/apps/" + g + "/capsule_231x87.jpg'/> \
          &nbsp;" + gameList[g] + " <i><small class='text-black-50'>(" + g + ")</small></i> \
        </a> \
        <span class='btn-group btn-group-sm float-right' role='group'> \
          <button type='button' class='btn btn-warning' id='game-edit' data-game='" + g + "'><i class='fas fa-edit' title='Edit'>&nbsp;</i></button> \
          <button type='button' class='btn btn-danger' id='game-remove' data-game='" + g + "'><i class='fas fa-trash' title='Remove'>&nbsp;</i></button> \
        </span> \
        </span> \
        <div class='input-group' id='edit-game-" + g + "' hidden> \
          <input class='form-control form-control-lg' type='text' placeholder='Paste Steampage URL or #num:name' id='edit-game-url'> \
          <div class='input-group-append'> \
            <button class='btn btn-success' type='button' id='button-edit-game' data-game='" + g + "'><i class='fas fa-edit'>&nbsp;</i>Edit</button> \
            <button class='btn btn-secondary' type='button' id='button-edit-cancel' data-game='" + g + "'><i class='fas fa-window-close'>&nbsp;</i>Cancel</button> \
          </div> \
        </div> \
      </li>"
      );
    }

    $("button#game-edit").click(function() {
      const game = $(this).data('game');
      var edit = $("#edit-game-" + game);
      edit.attr("hidden", false);
      edit.children("#edit-game-url").val(gameList[game]);
      $("#game-link-" + game).attr("hidden", true);
    });

    $("button#game-remove").click(function() {
      const game = $(this).data('game');
      delete gameList[game];
      RenderGamesList();
    });

    $("button#button-edit-cancel").click(function() {
      const game = $(this).data('game');
      $("#edit-game-" + game).attr("hidden", true);
      $("#game-link-" + game).attr("hidden", false);
      RenderGamesList();
    });

    $("button#button-edit-game").click(function() {
      const game = $(this).data('game');
      gameList[game] = $("#edit-game-" + game).children("#edit-game-url").val();
      $("#edit-game-" + game).attr("hidden", true);
      $("#game-link-" + game).attr("hidden", false);
      RenderGamesList();
    });
  }

  function AddGameRecord(record) {
    var pos = record.search(':');
    if (pos < 0) {
      console.log('invalid #num:name');
      return;
    }
    gameList[record.substring(1, pos)] = record.substring(pos+1);
  }

  function GameParse(editselector, buttonselector) {
    var record = $(editselector).val();
    if (record == undefined || record == '') {
      console.log('empty record for game');
      return;
    }

    $(editselector).val('');
    if (record[0] == '#') {
      AddGameRecord(record);
      RenderGamesList();
      return;
    }

    $(buttonselector).attr('disabled', true);
    fetch('/parsepage', {
      method: 'post',
      body: record,
    })
    .then(async function (response) {
      $(buttonselector).attr('disabled', false);
      const t = await response.text();
      AddGameRecord(t);
      RenderGamesList();
    }).catch(function (err) {
      $(buttonselector).attr('disabled', false);
      console.warn('Something went wrong.', err);
    });
  }

  // attach to gameslist buttons
  $('#button-add-game').click(function() {
    GameParse('input#new-game-url', this);
  });

  $('#save-games').click(function() {
    var button = this;
    $(button).attr('disabled', true);
    fetch('/savegames', {
      method: 'post',
      body: JSON.stringify(gameList),
    })
    .then(async function (response) {
      $(button).attr('disabled', false);
      if (response.ok) {
        $("#games-success-alert").attr("hidden", false);
      } else {
        $("#games-error-alert").attr("hidden", false);
      }
    }).catch(function (err) {
      $(button).attr('disabled', false);
      $("#games-error-alert").attr("hidden", false);
      console.error('Something went wrong.', err);
    });
  });

  function RenderCookiesList() {
    $("#cookie-edit").off();
    $("#cookie-remove").off();
    $("#button-edit-cookie-cancel").off();
    $("#button-edit-cookie").off();

    $('#cookies-div').empty();
    for (const c in cookiesList) {
      $('#cookies-div').append(
        "<li class='list-group-item bg-light'> \
          <div class='d-flex flex-row'> \
            <span class='w-25 mr-3'> " + c + "</span> \
            <div class='input-group'> \
              <input type='text' class='form-control' placeholder='Cookie params' disabled id='edit-cookie-" + c.hashCode() + "' value='" + cookiesList[c] + "'> \
              <span class='input-group-append'> \
                <button type='button' class='btn btn-warning' id='cookie-edit' data-cookie='" + c + "'><i class='fas fa-edit' title='Edit'>&nbsp;</i></button> \
                <button type='button' class='btn btn-danger' id='cookie-remove' data-cookie='" + c + "'><i class='fas fa-trash' title='Remove'>&nbsp;</i></button> \
                <button hidden type='button' class='btn btn-success' id='button-edit-cookie' data-cookie='" + c + "'><i class='fas fa-edit' title='Edit'>&nbsp;</i></button> \
                <button hidden type='button' class='btn btn-secondary' id='button-edit-cookie-cancel' data-cookie='" + c + "'><i class='fas fa-window-close' title='Cancel'>&nbsp;</i></button> \
              </span> \
            </div> \
          </div> \
        </li>"
      );
    }

    $("button#cookie-edit").click(function() {
      const cookie = $(this).data('cookie');
      $("#edit-cookie-" + cookie.hashCode()).attr("disabled", false);

      $(this).attr("hidden", true);
      $("#cookie-remove[data-cookie='" + cookie + "']").attr("hidden", true);
      $("#button-edit-cookie[data-cookie='" + cookie + "']").attr("hidden", false);
      $("#button-edit-cookie-cancel[data-cookie='" + cookie + "']").attr("hidden", false);
    });

    $("button#cookie-remove").click(function() {
      const cookie = $(this).data('cookie');
      delete cookiesList[cookie];
      RenderCookiesList();
    });

    $("button#button-edit-cookie-cancel").click(function() {
      const cookie = $(this).data('cookie');
      $(this).attr("hidden", true);
      $("#button-edit-cookie[data-cookie='" + cookie + "']").attr("hidden", true);
      $("#cookie-edit[data-cookie='" + cookie + "']").attr("hidden", false);
      $("#cookie-remove[data-cookie='" + cookie + "']").attr("hidden", false);
      RenderCookiesList();
    });

    $("button#button-edit-cookie").click(function() {
      const cookie = $(this).data('cookie');
      $(this).attr("hidden", true);
      $("#button-edit-cookie[data-cookie='" + cookie + "']").attr("hidden", true);
      $("#cookie-edit[data-cookie='" + cookie + "']").attr("hidden", false);
      $("#cookie-remove[data-cookie='" + cookie + "']").attr("hidden", false);
      cookiesList[cookie] = $("#edit-cookie-" + cookie.hashCode()).val();
      RenderCookiesList();
    });
  }

  // attach to cookies list buttons
  $('#button-add-cookie').click(function() {
    const name = $('#cookie-name').val();
    const value = $('#cookie-value').val() + ":" + $('#cookie-domain').val() + ":" + $('#cookie-path').val();
    cookiesList[name] = value;
    RenderCookiesList();
    $("input[id^=cookie-]").val("");
  });

  $('#save-cookies').click(function() {
    var button = this;
    $(button).attr('disabled', true);
    fetch('/savecookies', {
      method: 'post',
      body: JSON.stringify(cookiesList),
    })
    .then(async function (response) {
      $(button).attr('disabled', false);
      if (response.ok) {
        $("#cookies-success-alert").attr("hidden", false);
      } else {
        $("#cookies-error-alert").attr("hidden", false);
      }
    }).catch(function (err) {
      $(button).attr('disabled', false);
      $("#cookies-error-alert").attr("hidden", false);
      console.error('Something went wrong.', err);
    });
  });

  function RenderConfig() {
    $("#config-edit").off();
    $("#button-edit-config-cancel").off();
    $("#button-edit-config").off();
    $('#config-div').empty();
    var append = (name, value, parent) => {
      $('#config-div').append(
        "<li class='list-group-item bg-light'> \
          <div class='d-flex flex-row'> \
            <span class='w-25 mr-3'> " + name + "</span> \
            <div class='input-group'> \
              <input type='text' class='form-control' placeholder='Value' disabled id='edit-config-" + name.hashCode() + "' value='" + value + "'> \
              <span class='input-group-append'> \
                <button type='button' class='btn btn-warning' id='config-edit' data-name='" + name + "' data-parent='" + parent + "'><i class='fas fa-edit' title='Edit'>&nbsp;</i></button> \
                <button hidden type='button' class='btn btn-success' id='button-edit-config' data-name='" + name + "' data-parent='" + parent + "'><i class='fas fa-edit' title='Edit'>&nbsp;</i></button> \
                <button hidden type='button' class='btn btn-secondary' id='button-edit-config-cancel' data-name='" + name + "' data-parent='" + parent + "'><i class='fas fa-window-close' title='Cancel'>&nbsp;</i></button> \
              </span> \
            </div> \
          </div> \
        </li>"
      );
    }

    for (const c in config) {
      if (c == "mail") {
        for (const m in config[c]) {
          append(m, config[c][m], c);
        }
        continue;
      }
      append(c, config[c], "");
    }

    $("button#config-edit").click(function() {
      const name = $(this).data('name');
      $("#edit-config-" + name.hashCode()).attr("disabled", false);

      $(this).attr("hidden", true);
      $("#button-edit-config[data-name='" + name + "']").attr("hidden", false);
      $("#button-edit-config-cancel[data-name='" + name + "']").attr("hidden", false);
    });

    $("button#button-edit-config-cancel").click(function() {
      const name = $(this).data('name');
      $(this).attr("hidden", true);
      $("#config-edit[data-name='" + name + "']").attr("hidden", false);
      $("#button-edit-config[data-name='" + name + "']").attr("hidden", true);
      $("#edit-config-" + name.hashCode()).attr("disabled", true);
      RenderConfig();
    });

    $("button#button-edit-config").click(function() {
      const name = $(this).data('name');
      const p = $(this).data('parent');
      var value = $("#edit-config-" + name.hashCode()).val();
      if (name.includes("-num")) {
        value = parseInt(value);
      }
      if (p != undefined && p != "") {
        config[p][name] = value;
      } else {
        config[name] = value;
      }
      RenderConfig();
    });
  }

  // attach to cookies list buttons
  $('#save-config').click(function() {
    var button = this;
    $(button).attr('disabled', true);
    fetch('/saveconfig', {
      method: 'post',
      body: JSON.stringify(config),
    })
    .then(async function (response) {
      $(button).attr('disabled', false);
      if (response.ok) {
        $("#config-success-alert").attr("hidden", false);
      } else {
        $("#config-error-alert").attr("hidden", false);
      }
    }).catch(function (err) {
      $(button).attr('disabled', false);
      $("#config-error-alert").attr("hidden", false);
      console.error('Something went wrong.', err);
    });
  });

  RenderGamesList();
  RenderCookiesList();
  RenderConfig();
</script>
</body>
</html>

			{{end}}`
		)
		