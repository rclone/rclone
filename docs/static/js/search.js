function on_search() {
  document.search_form.q.value = document.search_form.words.value + " -site:forum.rclone.org";
  return true;
}
