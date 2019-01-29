function anchorJs() {
  if ($('.is-docs-page').length > 0) {
    anchors.options = {
      icon: '#'
    }

    anchors.add('.cli h2, .content h2, .content h3, .content h4');
  }
}

function scrollFadeInOut(threshold, element) {
  //element.hide();

  $(window).scroll(function() {
    if ($(this).scrollTop() > threshold) {
      element.fadeIn();
    } else {
      element.fadeOut();
    }
  });
}

function navbarScrollToggle() {
  const navbar = $('.is-home-page .navbar');
  const heroHeight = $('.hero').height();

  scrollFadeInOut(heroHeight, navbar);
}

$(function() {
  anchorJs();
  navbarScrollToggle();
});
