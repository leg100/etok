module "m1" {
  source = "../../outer/mods/m1"
}

module "m2" {
  source = "./inner/mods/m2"
}
