{function name="renderAlert"}
  {if $message}
    <div class="alert {$level}">{$message}</div>
  {else}
    <div class="alert">お知らせはありません。</div>
  {/if}
{/function}
{call name="renderAlert"}
