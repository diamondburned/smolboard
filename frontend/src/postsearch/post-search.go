package postsearch

type Controller interface {
	SearchPosts(input string)
}

type PostSearch struct {
	ctrl Controller
}

func NewPostSearch(ctrl Controller) *PostSearch {
	return &PostSearch{
		ctrl: ctrl,
	}
}
