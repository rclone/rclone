// Site JS

$(function() {
  // Add hover links on headings
  $("h2, h3, h4, h5, h6").each(function(i, el) {
    var $el, icon, id;
    $el = $(el);
    id = $el.attr('id');
    icon = '<i class="fa fa-link"></i>';
    if (id) {
      return $el.prepend($("<a />").addClass("header-link").attr("href", "#" + id).html(icon));
    }
  });
  // Wire up copy to clipboard buttons
  $(".copy-to-clipboard").click(function() {
    var copyText = $(this).prev();
    copyText.select();
    document.execCommand("copy");
    $(this).attr("title", "Copied!");
  });
});
