{extends file="../layouts/base.tpl"}
{block name="title"}ログイン{/block}
{block name="content"}
  {include file="../partials/header.tpl"}
  {include file="../partials/errors.tpl"}
  <form id="login-form" action="/login" data-label="a > b {$user.name}">
    <label>{$user.name}</label>
    <input name="email" value="{$user.email}">
    <button type="submit">ログイン</button>
  </form>
  {block name="form-help"}<p>入力内容を確認してください。</p>{/block}
{/block}
<script type="module" id="login-controller">
const loginLabel = `ログイン: ${"{$user.name}"}`;
function validateLogin(form) {
  const message = "literal </script> marker";
  return Boolean(form && message);
}
const submitLogin = (event) => event.preventDefault();
</script>
<script type="application/ld+json">
{"@context":"https://schema.org","name":"Login"}
</script>
<script src="../scripts/login.js"></script>
<script src="{$assetBase}/login.js"></script>
<script src="https://cdn.example.invalid/widget.js"></script>
<style>
.login-form { display: grid; gap: 0.5rem; }
</style>
{* {block name="fake"} *}
{literal}
  {$notSmarty}
  function literalExample() { return "}"; }
{/literal}
